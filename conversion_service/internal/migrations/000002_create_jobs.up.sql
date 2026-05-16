CREATE TABLE app_job (
  id               bigserial PRIMARY KEY,
  publicacion_id   bigint NOT NULL REFERENCES app_publicacion(id) ON DELETE CASCADE,
  calidad          varchar NOT NULL,
  estado           varchar NOT NULL,
  error            text NOT NULL DEFAULT '',
  requester_token  text NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT NOW(),
  updated_at       timestamptz NOT NULL DEFAULT NOW(),
  finished_at      timestamptz NULL,
  UNIQUE (publicacion_id, calidad)
);

CREATE INDEX idx_app_job_estado ON app_job (estado);
CREATE INDEX idx_app_job_requester_token ON app_job (requester_token);
