-- Create examples table for boilerplate demonstration
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS examples (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create updated_at trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- RunMigrations (internal/adapters/postgres/migrate.go) re-applies every
-- .sql file on every startup with ENABLE_AUTO_MIGRATE=true, no per-file
-- tracking table - every statement here must tolerate re-execution.
DROP TRIGGER IF EXISTS update_examples_updated_at ON examples;
CREATE TRIGGER update_examples_updated_at BEFORE UPDATE ON examples
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Insert sample data once; a plain INSERT would duplicate these rows on
-- every re-applied migration run.
INSERT INTO examples (name, description)
SELECT v.name, v.description
FROM (VALUES
    ('Sample Item 1', 'This is a sample example item for demonstration'),
    ('Sample Item 2', 'Another example item to show the structure')
) AS v(name, description)
WHERE NOT EXISTS (SELECT 1 FROM examples);
