package main

import (
    "encoding/json"
    "fmt"
    "log"
    "math/rand"
    "net/http"
    "sync/atomic"
    "time"

    "github.com/gin-gonic/gin"
)

const (
    Users           = 50
    ServiceURL      = "http://localhost:8080"
    VerifyPort      = ":3000"
    RequestInterval = 3 * time.Minute
)

var (
    requests uint64
    errors   uint64
    success  uint64
)

type JobResponse struct {
    JobID int64 `json:"job_id"`
}

type JobStatus struct {
    Status      string `json:"status"`
    DownloadURL string `json:"download_url"`
    Error       string `json:"error,omitempty"`
}

var (
    videos = []string{
        "video1",
        "2",
        "3",
        "4",
    }

    qualities = []string{
        "low",
        "medium",
        "high",
        "pro",
    }
)

func main() {
    go startVerifyServer()

    time.Sleep(time.Second)

    go printMetrics()

    log.Printf("starting %d fake users...\n", Users)

    for i := 0; i < Users; i++ {
        go simulateUser(i)
    }

    select {}
}

func startVerifyServer() {
    r := gin.Default()

    r.POST("/api/auth/check", func(c *gin.Context) {
        c.Status(http.StatusOK)
    })

    log.Printf("fake verify server listening on %s\n", VerifyPort)

    if err := r.Run(VerifyPort); err != nil {
        log.Fatal(err)
    }
}

func simulateUser(id int) {
    client := &http.Client{
        Timeout: 60 * time.Second,
    }

    for {
        video := randomVideo()
        quality := randomQuality()

        err := requestVideo(client, id, video, quality)
        if err != nil {
            atomic.AddUint64(&errors, 1)
            log.Printf("[user %d] error: %v\n", id, err)
        }

        sleep := RequestInterval + time.Duration(rand.Intn(3000))*time.Millisecond
        time.Sleep(sleep)
    }
}

func requestVideo(
    client *http.Client,
    userID int,
    videoID string,
    quality string,
) error {

    url := fmt.Sprintf(
        "%s/download/%s/%s",
        ServiceURL,
        videoID,
        quality,
    )

    req, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        return err
    }

    req.Header.Set(
        "Authorization",
        fmt.Sprintf("fake-user-%d", userID),
    )

    start := time.Now()

    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    atomic.AddUint64(&requests, 1)

    latency := time.Since(start)

    log.Printf(
        "[user %d] %s/%s -> %d (%v)",
        userID,
        videoID,
        quality,
        resp.StatusCode,
        latency,
    )

    switch resp.StatusCode {

    case http.StatusOK:
        atomic.AddUint64(&success, 1)
        return nil

    case http.StatusAccepted:
        var job JobResponse
        if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
            return err
        }
        return pollJob(client, userID, job.JobID)

    default:
        return fmt.Errorf("unexpected status: %d", resp.StatusCode)
    }
}

func pollJob(
    client *http.Client,
    userID int,
    jobID int64,
) error {

    for {
        url := fmt.Sprintf(
            "%s/jobs/%d",
            ServiceURL,
            jobID,
        )

        req, err := http.NewRequest(http.MethodGet, url, nil)
        if err != nil {
            return err
        }

        req.Header.Set(
            "Authorization",
            fmt.Sprintf("fake-user-%d", userID),
        )

        resp, err := client.Do(req)
        if err != nil {
            return err
        }

        var status JobStatus
        err = json.NewDecoder(resp.Body).Decode(&status)
        resp.Body.Close()
        if err != nil {
            return err
        }

        switch status.Status {

        case "completed":
            atomic.AddUint64(&success, 1)
            log.Printf("[user %d] job %d completed", userID, jobID)
            return nil

        case "failed":
            return fmt.Errorf("job failed: %s", status.Error)

        default:
            time.Sleep(2 * time.Second)
        }
    }
}

func printMetrics() {
    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
        reqs := atomic.LoadUint64(&requests)
        errs := atomic.LoadUint64(&errors)
        ok := atomic.LoadUint64(&success)

        log.Println("===================================")
        log.Printf("requests: %d\n", reqs)
        log.Printf("success : %d\n", ok)
        log.Printf("errors  : %d\n", errs)
        log.Println("===================================")
    }
}

func randomVideo() string {
    return videos[rand.Intn(len(videos))]
}

func randomQuality() string {
    return qualities[rand.Intn(len(qualities))]
}
