
CREATE TABLE IF NOT EXISTS rebellion_support (
	encampment_id UUID PRIMARY KEY REFERENCES encampments(id) ON DELETE CASCADE,
	total_contributed DOUBLE PRECISION DEFAULT 0
);
