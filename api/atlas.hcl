variable "dev_url" {
  type    = string
  default = "docker://postgres/16/dev?search_path=public"
}

data "external_schema" "judgify" {
  program = [
    "go",
    "run",
    "./cmd/atlas-schema",
  ]
  working_dir = "."
}

env "local" {
  src = data.external_schema.judgify.url
  dev = var.dev_url

  migration {
    dir = "file://internal/ent/migrate/migrations"
  }
}
