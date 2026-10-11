-- Seed plans FREE / PRO / BUSINESS (prix de départ, à ajuster après pilotes)
INSERT INTO plans (code, name, price_monthly_xof) VALUES
    ('FREE', 'Gratuit permanent', 0),
    ('PRO', 'Pro', 45000),
    ('BUSINESS', 'Business', 75000)
ON CONFLICT (code) DO NOTHING;
