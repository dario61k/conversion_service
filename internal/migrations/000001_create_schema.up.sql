CREATE TABLE app_publicacion (
  id             bigserial PRIMARY KEY,
  url_manifiesto text UNIQUE NOT NULL,
  descarga       jsonb NOT NULL
);

CREATE TABLE app_publicacion_lru (
  publicacion_id INT NOT NULL REFERENCES app_publicacion(id) ON DELETE CASCADE,
  calidad VARCHAR NOT NULL,
  lru TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(publicacion_id, calidad)
);
