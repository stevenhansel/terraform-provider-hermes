resource "hermes_cron_job" "task_check" {
  profile  = "default"
  name     = "task-check"
  prompt   = "Review my outstanding tasks and summarize the next actions."
  schedule = "every 1h"
  deliver  = "local"
}
