-- sqld:up
ALTER TABLE "public"."admissions" ADD COLUMN "completed_at" timestamptz;

-- sqld:down
ALTER TABLE "public"."admissions" DROP COLUMN "completed_at";
