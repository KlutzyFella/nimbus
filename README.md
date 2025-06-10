# ☁️ Nimbus — Image to URL Converter

Nimbus is a minimal web app that lets users upload images and instantly get a publicly shareable URL. Built with Next.js, Clerk, and a Go-powered AWS Lambda, it provides a seamless interface to upload files to S3 and retrieve a link.

### ⚡ What It Does
- Authenticated image uploads (via Clerk)
- Uploads files to Amazon S3 via a Go Lambda function
- Returns a public URL for the uploaded file
- Built for fast sharing and simple usage

### Tech Stack
- Next.js
- Tailwind CSS
- Clerk
- AWS Lambda (Go)
- Amazon S3
- Shadcn UI & Acternity UI

### 📁 File Upload Flow
1. User selects or drags an image file.
2. The file is POSTed to your AWS Lambda endpoint.
3. Lambda (written in Go) stores the image in S3 and returns a JSON with the public image URL.
4. The client displays the URL to the user.

### Current work progress and ideas: 

As of 7/6/2025:
- I setup the basic frontend UI and Auth (using Clerk)
- Wrote an async function to handle file upload that sends a POST request to the AWS Lambda endpoint. 
- I am gonna work on the Go function next 