package worker

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/dario61k/conversion-service/internal/config"
	"github.com/dario61k/conversion-service/internal/db"
	"github.com/dario61k/conversion-service/internal/domain"
	"github.com/dario61k/conversion-service/internal/queue"
	"github.com/dario61k/conversion-service/internal/services"
	"github.com/dario61k/conversion-service/internal/storage"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Runner struct {
	wg    sync.WaitGroup
	errCh chan error
}

func Start(ctx context.Context, cfg config.Config, repo *db.Repository, store *storage.S3, dl *services.Downloader, q *queue.Client) *Runner {
	runner := &Runner{errCh: make(chan error, cfg.WorkerCount)}

	if cfg.WorkerCount <= 0 {
		close(runner.errCh)
		return runner
	}

	for i := 0; i < cfg.WorkerCount; i++ {
		workerID := i + 1
		runner.wg.Add(1)
		go func() {
			defer runner.wg.Done()
			if err := runWorker(ctx, workerID, cfg, repo, store, dl, q); err != nil {
				select {
				case runner.errCh <- err:
				default:
				}
			}
		}()
	}

	return runner
}

func (r *Runner) Wait() {
	r.wg.Wait()
	close(r.errCh)
}

func (r *Runner) Errors() <-chan error {
	return r.errCh
}

func runWorker(ctx context.Context, workerID int, cfg config.Config, repo *db.Repository, store *storage.S3, dl *services.Downloader, q *queue.Client) error {
	consumer, err := q.NewConsumer()
	if err != nil {
		return err
	}
	defer consumer.Close()

	log.Printf("worker-%d ready", workerID)

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-consumer.Deliveries():
			if !ok {
				return nil
			}
			if err := handleDelivery(ctx, cfg, repo, store, dl, q, delivery); err != nil {
				log.Printf("worker-%d error: %v", workerID, err)
			}
		}
	}
}

func handleDelivery(ctx context.Context, cfg config.Config, repo *db.Repository, store *storage.S3, dl *services.Downloader, q *queue.Client, delivery amqp.Delivery) error {
	msg, err := q.ParseMessage(delivery)
	if err != nil {
		_ = delivery.Reject(false)
		return err
	}

	_ = repo.UpdateJobStatus(ctx, msg.JobID, domain.JobProcessing, "")

	if store.Exists(ctx, cfg.DownloadsBucket, msg.ObjectKey) {
		_ = repo.UpdateJobStatus(ctx, msg.JobID, domain.JobCompleted, "")
		_ = delivery.Ack(false)
		return nil
	}

	err = dl.BuildVideo(ctx, msg.Manifest, msg.Quality, msg.ObjectKey)
	if err == nil {
		_ = repo.UpdateJobStatus(ctx, msg.JobID, domain.JobCompleted, "")
		_ = delivery.Ack(false)
		return nil
	}

	if errors.Is(err, services.ErrNotFound) {
		_ = repo.UpdateJobStatus(ctx, msg.JobID, domain.JobFailed, err.Error())
		_ = q.PublishDLQ(ctx, msg, "missing source media")
		_ = delivery.Ack(false)
		return err
	}

	if msg.Attempt >= cfg.AMQPMaxAttempts {
		_ = repo.UpdateJobStatus(ctx, msg.JobID, domain.JobFailed, err.Error())
		_ = q.PublishDLQ(ctx, msg, "max attempts reached")
		_ = delivery.Ack(false)
		return err
	}

	msg.Attempt++
	_ = repo.UpdateJobStatus(ctx, msg.JobID, domain.JobQueued, err.Error())
	_ = q.PublishRetry(ctx, msg)
	_ = delivery.Ack(false)
	return err
}
