# Nimbus V2 platform infrastructure (EC2 host + IAM + security group).
#
# Scope: this file manages the K8s host only. It does NOT create the S3
# upload bucket, the EC2 key pair, or any V1 (Lambda/API Gateway/SQS)
# resources — those are manual prerequisites, see README.md.

terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

variable "aws_region" {
  description = "AWS region for all resources."
  type        = string
  default     = "us-east-1"
}

variable "allowed_ssh_cidr" {
  description = "CIDR block allowed to SSH (port 22). Set to your own IP/32; never commit a value."
  type        = string
}

variable "key_name" {
  description = "Name of an existing EC2 key pair. Terraform does not create it."
  type        = string
}

variable "s3_bucket_name" {
  description = "Name of the existing S3 upload bucket. Terraform does not create it."
  type        = string
  default     = "nimbus.uploads"
}

variable "ami" {
  description = "Ubuntu 22.04 AMI for the chosen region."
  type        = string
  default     = "ami-053b0d53c279acc90" # us-east-1
}

variable "instance_type" {
  description = "EC2 size. 2GB RAM is the observed minimum for K3s (see README)."
  type        = string
  default     = "t3.small"
}

provider "aws" {
  region = var.aws_region
}

# Define the IAM Role for S3 Access
resource "aws_iam_role" "ec2_s3_role" {
  name = "ec2-s3-access-role"

  # This policy allows EC2 to assume this role
  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [{
      Action    = "sts:AssumeRole",
      Effect    = "Allow",
      Principal = { Service = "ec2.amazonaws.com" }
    }]
  })
}

# Least-privilege write access: the uploader only ever calls PutObject
# on keys in the upload bucket. (Previously AmazonS3FullAccess.)
resource "aws_iam_role_policy" "uploader_s3_write" {
  name = "uploader-s3-write"
  role = aws_iam_role.ec2_s3_role.id

  policy = jsonencode({
    Version = "2012-10-17",
    Statement = [{
      Effect   = "Allow",
      Action   = ["s3:PutObject"],
      Resource = "arn:aws:s3:::${var.s3_bucket_name}/*"
    }]
  })
}

# Create the Instance Profile
resource "aws_iam_instance_profile" "ec2_profile" {
  name = "ec2-s3-access-profile"
  role = aws_iam_role.ec2_s3_role.name
}

# Define the Security Group/Firewall
resource "aws_security_group" "k8s_sg" {
  name        = "k8s-server-sg"
  description = "Allow SSH, HTTP, and HTTPS"

  # Allow SSH (Port 22) - restricted to your IP via allowed_ssh_cidr
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.allowed_ssh_cidr]
  }

  # Allow HTTP (Port 80)
  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Allow HTTPS (Port 443)
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Allow all outbound traffic
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# Define the EC2 Instance Itself
resource "aws_instance" "k8s_server" {
  ami           = var.ami
  instance_type = var.instance_type

  iam_instance_profile   = aws_iam_instance_profile.ec2_profile.name
  vpc_security_group_ids = [aws_security_group.k8s_sg.id]
  key_name               = var.key_name

  # Run user-data.sh on first boot
  user_data = file("user-data.sh")

  tags = {
    Name = "K8s-Server (Terraform)"
  }
}
