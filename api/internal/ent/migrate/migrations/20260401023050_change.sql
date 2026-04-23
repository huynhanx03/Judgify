-- Create index "submission_problem_id" to table: "submissions"
CREATE INDEX "submission_problem_id" ON "public"."submissions" ("problem_id");
-- Create index "submission_user_id" to table: "submissions"
CREATE INDEX "submission_user_id" ON "public"."submissions" ("user_id");
-- Create index "submission_user_id_problem_id" to table: "submissions"
CREATE INDEX "submission_user_id_problem_id" ON "public"."submissions" ("user_id", "problem_id");
