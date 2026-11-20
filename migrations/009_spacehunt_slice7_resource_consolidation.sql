
DO $$
BEGIN
	IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='energy')
	   AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='electricity') THEN
		ALTER TABLE resources RENAME COLUMN energy TO electricity;
	END IF;
	IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='steel')
	   AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='metal') THEN
		ALTER TABLE resources RENAME COLUMN steel TO metal;
	END IF;
	IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='uranium')
	   AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='crystal') THEN
		ALTER TABLE resources RENAME COLUMN uranium TO crystal;
	END IF;
END $$;

DO $$
BEGIN
	IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='iron') THEN
		UPDATE resources SET metal = metal + COALESCE(iron, 0) + COALESCE(oil, 0);
		ALTER TABLE resources DROP COLUMN IF EXISTS iron;
		ALTER TABLE resources DROP COLUMN IF EXISTS oil;
	END IF;
	IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='resources' AND column_name='diamond') THEN
		UPDATE resources SET crystal = crystal + COALESCE(diamond, 0) + COALESCE(gold, 0) + COALESCE(silver, 0);
		ALTER TABLE resources DROP COLUMN IF EXISTS diamond;
		ALTER TABLE resources DROP COLUMN IF EXISTS gold;
		ALTER TABLE resources DROP COLUMN IF EXISTS silver;
	END IF;
END $$;

