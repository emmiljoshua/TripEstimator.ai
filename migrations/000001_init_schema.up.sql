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