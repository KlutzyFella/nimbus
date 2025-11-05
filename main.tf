# Configure the AWS Provider
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = "us-east-1" 
}

# Define the IAM Role for S3 Access
resource "aws_iam_role" "ec2_s3_role" {
  name = "ec2-s3-access-role"

  # This policy allows EC2 to assume this role
  assume_role_policy = jsonencode({
    Version = "2012-10-17",
    Statement = [{
      Action = "sts:AssumeRole",
      Effect = "Allow",
      Principal = { Service = "ec2.amazonaws.com" }
    }]
  })
}

# Attach the S3 Policy to the Role
resource "aws_iam_role_policy_attachment" "s3_access" {
  role       = aws_iam_role.ec2_s3_role.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonS3FullAccess"
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

  # Allow SSH (Port 22) - restricted to your IP
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["73.161.253.179/32"] 
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
  ami           = "ami-053b0d53c279acc90" # common Ubuntu 22.04 AMI for us-east-1
  instance_type = "t3.small"

  iam_instance_profile   = aws_iam_instance_profile.ec2_profile.name
  vpc_security_group_ids = [aws_security_group.k8s_sg.id]
  key_name               = "nimbus-keyPair"
  
  # Run user-data.sh on first boot
  user_data = file("user-data.sh")

  tags = {
    Name = "K8s-Server (Terraform)"
  }
}