resource "aws_cloudwatch_log_group" "function_log_group" {
  name              = format("/aws/lambda/%s", aws_lambda_function.asg_scale.function_name)
  retention_in_days = 7
  lifecycle {
    prevent_destroy = false
  }
}

resource "aws_iam_role" "asg_scale" {
  name = format("lambda-asg-scale-%s", var.region)

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })

  inline_policy {
    name = "asg_scale"

    policy = jsonencode({
      "Version" : "2012-10-17",
      "Statement" : [
        {
          Sid : "AutoScaling",
          Action : [
            "autoscaling:DescribeAutoScalingGroups",
            "autoscaling:UpdateAutoScalingGroup",
            "autoscaling:CreateOrUpdateTags",
            "autoscaling:DeleteTags"
          ],
          Effect : "Allow",
          Resource : "*"
        }
      ]
    })
  }
}

resource "aws_iam_role_policy_attachment" "asg_scale_basic" {
  role       = aws_iam_role.asg_scale.id
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "null_resource" "build_source" {
  provisioner "local-exec" {
    command = "GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o ${path.module}/out/bootstrap ${path.module}/source"
  }

  triggers = {
    asg  = filemd5("${path.module}/source/asg.go")
    aws  = filemd5("${path.module}/source/aws.go")
    main = filemd5("${path.module}/source/main.go")
    tag  = filemd5("${path.module}/source/tag.go")
  }
}

data "archive_file" "asg_scale" {
  type        = "zip"
  source_file = "${path.module}/out/bootstrap"
  output_path = "${path.module}/out/bootstrap.zip"
  depends_on  = [null_resource.build_source]
}

resource "aws_lambda_function" "asg_scale" {
  function_name    = "asg-scale"
  filename         = data.archive_file.asg_scale.output_path
  source_code_hash = data.archive_file.asg_scale.output_base64sha256
  role             = aws_iam_role.asg_scale.arn
  architectures    = ["arm64"]
  runtime          = "provided.al2023"
  handler          = "bootstrap"
  timeout          = 10

  environment {
    variables = {
      ASG_PREFIX = join(",", var.asg_prefixes)
    }
  }
}
