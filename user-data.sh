#!/bin/bash

sleep 10

# Install K3s (disabling Traefik)
curl -sfL https://get.k3s.io | sh -s - --disable=traefik
sleep 10

# Install the Nginx Ingress Controller
/usr/local/bin/k3s kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/baremetal/deploy.yaml

# Create a .bash_aliases file for the ubuntu user
# Easier to use kubectl when you SSH in
echo "alias kubectl='sudo k3s kubectl'" >> /home/ubuntu/.bash_aliases
chown ubuntu:ubuntu /home/ubuntu/.bash_aliases