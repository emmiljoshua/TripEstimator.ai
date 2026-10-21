BEGIN;

INSERT INTO trips (trip_name, start_date, end_date, party_size, origin_city, origin_country)
VALUES ('Japan 2026', '2026-09-14', '2026-09-25', 6, 'Austin', 'US');

INSERT INTO trip_segments (trip_id, city_stayed, country_stayed, start_date, end_date) VALUES
    ((SELECT id FROM trips WHERE trip_name = 'Japan 2026'), 'Tokyo', 'JP', '2026-09-14', '2026-09-18'),
    ((SELECT id FROM trips WHERE trip_name = 'Japan 2026'), 'Osaka', 'JP', '2026-09-18', '2026-09-22'),
    ((SELECT id FROM trips WHERE trip_name = 'Japan 2026'), 'Tokyo', 'JP', '2026-09-22', '2026-09-25');

INSERT INTO expenses (trip_segment_id, category_id, description, amount, currency, amount_in_usd, spent_on)
SELECT seg.id, cat.id, e.description, e.amount, 'USD', e.amount, seg.start_date
FROM (VALUES
    ('2026-09-14', 'lodging',             'Tokyo Airbnb',                         791.02),
    ('2026-09-18', 'lodging',             'Osaka hotel',                          370.42),
    ('2026-09-22', 'lodging',             'Tokyo hotel',                         1191.42),
    ('2026-09-18', 'intercity_transport', 'Shinkansen Tokyo to Osaka, 6 people',  562.03),
    ('2026-09-22', 'intercity_transport', 'Shinkansen Osaka to Tokyo, 6 people',  562.03),
    ('2026-09-14', 'flights',             'AUS, 1 person, miles plus taxes',      106.82),
    ('2026-09-14', 'flights',             'IAD, 3 people',                       2927.79),
    ('2026-09-14', 'flights',             'MNL, 2 people',                        300.00)
) AS e(segment_start, category, description, amount)
JOIN trip_segments seg ON seg.start_date = e.segment_start::date
JOIN trips t           ON t.id = seg.trip_id AND t.trip_name = 'Japan 2026'
JOIN categories cat    ON cat.name = e.category;

COMMIT;