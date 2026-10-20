CREATE TABLE trips (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id        BIGINT,
    trip_name      TEXT NOT NULL,
    start_date     DATE NOT NULL,
    end_date       DATE NOT NULL,
    party_size     INT NOT NULL CHECK (party_size > 0),
    origin_city    TEXT NOT NULL,
    origin_country TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_date >= start_date)
);

CREATE TABLE trip_segments (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    trip_id        BIGINT NOT NULL REFERENCES trips(id) ON DELETE CASCADE, -- has to match a real trips.id. If the trip is deleted, then these go with it
    start_date     DATE NOT NULL,
    end_date       DATE NOT NULL,
    nights         INT GENERATED ALWAYS AS (end_date - start_date) STORED,
    city_stayed    TEXT NOT NULL,
    country_stayed TEXT NOT NULL,
    CHECK (end_date >= start_date)
);

CREATE TABLE categories (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name                TEXT NOT NULL UNIQUE,
    normalization_unit  TEXT NOT NULL CHECK (normalization_unit IN ('per_person_per_day', 'per_person_per_trip', 'per_night'))
);

CREATE TABLE expenses (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    trip_segment_id     BIGINT NOT NULL REFERENCES trip_segments(id) ON DELETE CASCADE, -- has to match a real trip_segments.id. If the trip_segment is deleted, then these go with it
    category_id         BIGINT NOT NULL REFERENCES categories(id), -- don't delete on cascade, to preserve the historical data. If a category is deleted, we can just set it to "deleted" in the future.
    description         TEXT,
    amount              NUMERIC(10, 2) NOT NULL CHECK (amount >= 0),
    currency            TEXT NOT NULL CHECK (length(currency) = 3), -- ISO 4217 currency code
    amount_in_usd       NUMERIC(10, 2) NOT NULL CHECK (amount_in_usd >= 0),
    spent_on            DATE NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE fx_rates (
    currency            TEXT NOT NULL CHECK (length(currency) = 3), -- ISO 4217 currency code
    usd_per_unit        NUMERIC(18, 8) NOT NULL CHECK (usd_per_unit > 0),
    rate_date           DATE NOT NULL,
    PRIMARY KEY (currency, rate_date)
);

CREATE TABLE estimates (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id             BIGINT,
    trip_id             BIGINT REFERENCES trips(id) ON DELETE CASCADE, -- has to match a real trips.id. If the trip is deleted, then these go with it
    request             JSONB NOT NULL,
    result              JSONB NOT NULL,
    model_version       TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);