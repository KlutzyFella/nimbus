# ☁️ Nimbus — A Cloud-Native App

**Nimbus** is a full-stack "Image-to-URL" application that was built in two phases, documenting a real-world journey from a simple serverless model to a fully-managed, self-hosted platform on Kubernetes.

The project consists of a [Next.js](https://nextjs.org/) frontend, a [Go](https://go.dev/) backend, and two distinct, deployable architectures.

<u> Note: This repository contains two versions of the same application (V1 and V2), representing two different infrastructure philosophies</u>

### **V1 - The Serverless Application**

The original version of Nimbus. Designed for rapid development, zero server management, and low cost at small scale.

  * **Frontend:** Next.js, Tailwind CSS, Clerk (for auth), Shadcn UI
  * **Backend:** AWS Lambda (Go) & API Gateway
  * **Storage:** Amazon S3 (object storage; there is no database in this project)
  * **Flow:** The Next.js app calls an API Gateway endpoint, which triggers a Go Lambda function. The function uploads the file to S3 and returns the URL. It also sends the object key to SQS; the processor Lambda currently only logs the message — no image transformation is implemented yet.

-----

### **V2 - The Cloud-Native Platform**

This is the advanced version, built to learn and demonstrate the fundamentals of infrastructure, orchestration, and networking that managed services hide. The goal was to build the *platform* itself.

  * **Application:** Go microservices (`net/http` servers). The processor
      service currently logs the notification and replies OK — it performs
      no image processing yet (see Roadmap).
  * **Containerization:** Docker
  * **Orchestration:** Kubernetes (K3s)
  * **Cloud:** AWS EC2 (t3.small), S3, IAM Roles
  * **Networking:** Nginx Ingress Controller

-----

## 🛠️ Tech Stack (V2 Platform)

  * **Backend:** Go (`net/http`)
  * **Containerization:** Docker
  * **Orchestration:** Kubernetes (K3s)
  * **Reverse Proxy / Ingress:** Nginx
  * **Cloud Provider:** AWS
      * **Compute:** EC2 (t3.small VM)
      * **Storage:** S3 (for file storage)
      * **Security:** IAM Roles, Security Groups

## 🧠 V2 Deployment: Key Challenges & Debugging

This project's value is in the real-world infrastructure problems I had to solve to migrate from V1 to V2.

  * **Challenge: EC2 `Out of Memory` Errors**

      * **Problem:** My initial `t2.micro` (1GB RAM) EC2 instance was too small for a K8s cluster. This caused the Linux kernel to `OOMKill`critical processes, including the K8s DNS (`coredns`), leading to `Connection timed out` errors.
      * **Debug Process:** I verified Security Groups and IP, then used the **EC2 Instance Screenshot** tool to find the kernel-level "Out of memory" error messages.
      * **Solution:** Re-provisioned the cluster on a `t3.small` (2GB RAM) instance, which stabilized the environment.

  * **Challenge: Cloud Authentication (IAM)**

      * **Problem:** My `uploader` pod couldn't authenticate to S3.
      * **Debug Process:** I first tried mounting local `~/.aws` files, but `kubectl describe pod` revealed a `failed to fulfil mount request` error, proving the K8s node (a container itself) couldn't see my laptop's filesystem.
      * **Solution:** I implemented the secure, professional solution by creating an **IAM Role** with S3 permissions and attaching it directly to the EC2 instance. The Go SDK automatically detected these credentials.

  * **Challenge: K8s Ingress (404 Not Found)**

      * **Problem:** All requests to my public IP returned a `404`.
      * **Debug Process:** I used `kubectl logs -f` on the Nginx pod but saw no new requests, proving traffic wasn't even reaching it.
      * **Solution:** Discovered the default K3s ingress controller (`Traefik`) was in conflict. I had to **re-install K3s with Traefik disabled** to allow Nginx to correctly bind to port 80.

  * **Challenge: K8s Service DNS (500 Error)**

      * **Problem:** The `uploader` pod could be reached but returned a `500` error.
      * **Debug Process:** `kubectl logs` on the `uploader` pod showed a `server misbehaving` DNS error.
      * **Solution:** Discovered the Go code was trying to connect to the hostname `worker`, but the Kubernetes `Service` was named `worker-service`. I learned that **inter-pod communication must use K8s Service DNS names**.

## 🚀 Roadmap: Automating the Platform

This project is the foundation for a fully automated platform.

  * **1. Infrastructure as Code (Terraform, partial)**

      * **Status:** `main.tf` provisions the EC2 host, IAM role, and
        security group. It does **not** manage the S3 bucket, the EC2
        key pair, or any V1 (Lambda/API Gateway/SQS) resources — those
        are manual prerequisites (see below).

  * **2. CI/CD Pipeline (GitHub Actions)**

      * **Goal:** Create a zero-touch "code-to-cluster" pipeline. On `git push`, a GitHub Action will:
        1.  Build and push the Go binaries to Docker Hub.
        2.  SSH into the EC2 instance and run `kubectl apply` to deploy the new version.

  * **3. Resilient Microservices (Message Queue)**

      * **Goal:** Replace the synchronous HTTP call between services with a true message queue (like **RabbitMQ** or **NATS**) deployed *inside* the K8s cluster for superior decoupling and reliability.

  * **4. Observability (Prometheus + Grafana)**

      * **Goal:** Deploy a full monitoring stack to create a Grafana dashboard visualizing application health and performance.

## ⚙️ How to Deploy (V2 Platform)

> Honesty note: the EC2/K3s steps below are the author's original
> procedure and were **not** re-run in the hardening pass (they need
> paid AWS resources). The build, test, and Docker commands were run
> and are exact.

### Prerequisites (all manual — Terraform does not create these)

  * An S3 bucket for uploads (the manifests default to `nimbus.uploads`).
    Nothing in this repo creates it — create it first.
  * An EC2 key pair matching `key_name` in `main.tf` (or change the value).
  * A free Clerk application for the frontend (see `frontend/.env.example`).
  * A Docker Hub account; the image name must be identical in
    `deployment.yaml` (currently `klutzyfella/...`) and the
    `DOCKERHUB_USERNAME` CI secret, or pods will pull the wrong image.

### Configuration

| Variable | Where | Required | Notes |
|---|---|---|---|
| `S3_BUCKET_NAME` | uploader container / Lambda env | yes | Must already exist |
| `SQS_QUEUE_URL` | uploader Lambda env | yes (V1) | Only the Lambda uses SQS |
| `WORKER_URL` | uploader container env | no | Defaults to `http://worker-service:8081/process` |
| `NEXT_PUBLIC_UPLOAD_ENDPOINT` | `frontend/.env.local` | yes | e.g. `http://<EC2_PUBLIC_IP>/upload` |
| `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY` | `frontend/.env.local` | yes at runtime | Not needed for `npm run build` |

CI secrets (`Settings → Secrets`): `DOCKERHUB_USERNAME`,
`DOCKERHUB_TOKEN`, `EC2_HOST_IP`, `EC2_USER`, `EC2_SSH_KEY`.
Note the CI pipeline only runs `rollout restart` — after changing a
manifest you must still `kubectl apply -f` it yourself.

### How to Run the Tests (verified)

  * Go services: `go test ./...` inside `image-uploader/`,
    `image-processor/`, `image-uploader-lambda/`,
    `image-processor-lambda/` (real unit tests exist for the two
    uploaders; the processors have none yet).
  * Frontend: `npm install && npm run build` inside `frontend/`
    (needs no keys; running it needs the Clerk keys above).

### Deploy Steps

1.  **Build & Push Docker Images** (run inside each service directory,
    e.g. `cd image-uploader`):

      * `docker build -t <your-dockerhub-id>/image-uploader .`
      * `docker push <your-dockerhub-id>/image-uploader`
      * Repeat from `image-processor/` for `image-processor`

2.  **Launch EC2 & Install K3s:**

      * Launch a `t3.small` (2GB RAM) instance.
      * Attach an IAM Role with S3 write access to your upload bucket
        (the manifests rely on the EC2 role — no credential files).
      * Configure Security Group to allow ports 22 (SSH) and 80 (HTTP).
      * SSH in and run:
        ```bash
        # Install K3s without the default ingress
        curl -sfL https://get.k3s.io | sh -s - --disable=traefik

        # Install Nginx Ingress Controller
        sudo k3s kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/baremetal/deploy.yaml
        ```

3.  **Deploy the Application:**

      * Set `S3_BUCKET_NAME` and your Docker Hub image names in
        `deployment.yaml`.
      * Copy the files to your server (`scp`).
      * Apply the manifests:
        ```bash
        sudo k3s kubectl apply -f deployment.yaml
        sudo k3s kubectl apply -f ingress.yaml
        ```

4.  **Test:**

      * Copy `frontend/.env.example` to `frontend/.env.local` and set
        `NEXT_PUBLIC_UPLOAD_ENDPOINT=http://<YOUR_EC2_PUBLIC_IP>/upload`
        plus your Clerk keys.
      * `cd frontend && npm install && npm run dev`, open the printed
        URL, and upload a file.