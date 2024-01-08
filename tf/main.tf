provider "aws" {
  region = var.region
}

variable "region" {
  description = "AWS Region"
  default     = "eu-west-2"
}

variable "asg_prefixes" {
  description = "Prefixes of autoscaling groups that will be scaled based on schedule"
  default     = ["eks-default-"]
}

variable "scale_down_cron" {
  description = "Cron schedule expression for scale down"
  default     = "0 22 ? * MON-FRI *"
}

variable "scale_up_cron" {
  description = "Cron schedule expression for scale up"
  default     = "0 6 ? * MON-FRI *"
}


resource "aws_iam_role" "asg_scale_schedule" {
  name = format("lambda-asg-scale-schedule-%s", var.region)

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "scheduler.amazonaws.com"
        }
      },
    ]
  })

  inline_policy {
    name = "asg_scale"

    policy = jsonencode({
      "Version" : "2012-10-17",
      "Statement" : [
        {
          Sid : "InvokeLambda",
          Action : [
            "lambda:InvokeFunction"
          ],
          Effect : "Allow",
          Resource : "*"
        }
      ]
    })
  }
}

resource "aws_scheduler_schedule" "asg_scale_down" {
  name       = "asg-scale-down"
  group_name = "default"

  flexible_time_window {
    mode = "OFF"
  }

  schedule_expression = format("cron(%s)", var.scale_down_cron)

  target {
    arn      = aws_lambda_function.asg_scale.arn
    role_arn = aws_iam_role.asg_scale_schedule.arn

    input = jsonencode({
      Payload = "scale-down"
    })
  }
}

resource "aws_scheduler_schedule" "asg_scale_up" {
  name       = "asg-scale-up"
  group_name = "default"

  flexible_time_window {
    mode = "OFF"
  }

  schedule_expression = format("cron(%s)", var.scale_up_cron)

  target {
    arn      = aws_lambda_function.asg_scale.arn
    role_arn = aws_iam_role.asg_scale_schedule.arn

    input = jsonencode({
      Payload = "scale-up"
    })
  }
}
