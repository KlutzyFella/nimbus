# ☁️ Nimbus — A Cloud-Native App

**Nimbus** is a full-stack "Image-to-URL" application that was built in two phases, documenting a real-world journey from a simple serverless model to a fully-managed, self-hosted platform on Kubernetes.

The project consists of a [Next.js](https://nextjs.org/) frontend, a [Go](https://go.dev/) backend, and two distinct, deployable architectures.

<u> Note: This repository contains two versions of the same application (V1 and V2), representing two different infrastructure philosophies</u>

### **V1 - The Serverless Application**

The original version of Nimbus. Designed for rapid development, zero server management, and low cost at small scale.

  * **Frontend:** Next.js, Tailwind CSS, Clerk (for auth), Shadcn UI
  * **Backend:** AWS Lambda (Go) & API Gateway
  * **Database:** Amazon S3
  * **Flow:** The Next.js app calls an API Gateway endpoint, which triggers a Go Lambda function. The function uploads the file to S3 and returns the URL.

[Image of a serverless architecture diagram to be exported here from figma]

-----

### **V2 - The Cloud-Native Platform**

This is the advanced version, built to learn and demonstrate the fundamentals of infrastructure, orchestration, and networking that managed services hide. The goal was to build the *platform* itself.

  * **Application:** Go microservices (`net/http` servers)
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

  * **1. (In Progress) Infrastructure as Code (Terraform)**

      * **Goal:** Automate the creation of the entire cloud infrastructure (EC2, IAM Roles, Security Groups, etc.) using Terraform.

  * **2. CI/CD Pipeline (GitHub Actions)**

      * **Goal:** Create a zero-touch "code-to-cluster" pipeline. On `git push`, a GitHub Action will:
        1.  Build and push the Go binaries to Docker Hub.
        2.  SSH into the EC2 instance and run `kubectl apply` to deploy the new version.

  * **3. Resilient Microservices (Message Queue)**

      * **Goal:** Replace the synchronous HTTP call between services with a true message queue (like **RabbitMQ** or **NATS**) deployed *inside* the K8s cluster for superior decoupling and reliability.

  * **4. Observability (Prometheus + Grafana)**

      * **Goal:** Deploy a full monitoring stack to create a Grafana dashboard visualizing application health and performance.

## ⚙️ How to Deploy (V2 Platform)

1.  **Build & Push Docker Images:**

      * `docker build -t your-id/image-uploader .`
      * `docker push your-id/image-uploader`
      * Repeat for `image-processor`

2.  **Launch EC2 & Install K3s:**

      * Launch a `t3.small` (2GB RAM) instance.
      * Attach an IAM Role with `AmazonS3FullAccess`.
      * Configure Security Group to allow ports 22 (SSH) and 80 (HTTP).
      * SSH in and run:
        ```bash
        # Install K3s without the default ingress
        curl -sfL https://get.k3s.io | sh -s - --disable=traefik

        # Install Nginx Ingress Controller
        sudo k3s kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/baremetal/deploy.yaml
        ```

3.  **Deploy the Application:**

      * Update `deployment.yaml` and `ingress.yaml` with your correct Docker Hub image names.
      * Copy the files to your server (`scp`).
      * Apply the manifests:
        ```bash
        sudo k3s kubectl apply -f deployment.yaml
        sudo k3s kubectl apply -f ingress.yaml
        ```

4.  **Test:**

      * Find your EC2 instance's public IP and update your Next.js app's `.env.local` file to point to `http://<YOUR_EC2_PUBLIC_IP>/upload`.
      * Run the frontend and upload a file.