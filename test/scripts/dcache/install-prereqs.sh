#!/bin/bash
#
# Install docker-ce, kind, kubectl and helm on the ADO agent, idempotently.
# The dist_cache nightly E2E stage runs on `kind` (Kubernetes IN Docker); see
# test/scripts/dcache/PLAN.md for the rationale for choosing kind over the
# minikube path that vienna-tachyon publishes for.

set -e

# Detect architecture (supports amd64, arm64)
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
  KIND_ARCH="amd64"
  KUBE_ARCH="amd64"
elif [ "$ARCH" = "aarch64" ]; then
  KIND_ARCH="arm64"
  KUBE_ARCH="arm64"
else
  echo "Unsupported architecture: $ARCH" >&2
  exit 1
fi

# Pin kind to a released version so upgrades to `kind latest` don't silently
# change our cluster substrate. Bump intentionally when we want the newer
# kindest/node images.
KIND_VERSION="${KIND_VERSION:-v0.24.0}"

# Function to check if a command exists
check_command() {
  command -v "$1" >/dev/null 2>&1
}

# Install Docker CE (Ubuntu's docker.io is stuck on 24.x / API 1.43, incompatible with newer daemons)
if ! check_command docker || ! docker buildx version &>/dev/null; then
  echo "Installing Docker CE..."
  sudo apt-get remove -y docker.io containerd runc moby-tini moby-engine moby-cli moby-compose moby-containerd moby-runc moby-buildx 2>/dev/null || true
  sudo install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg --yes
  sudo chmod a+r /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
    | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
  sudo apt-get update
  sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin
  sudo systemctl enable docker
  sudo systemctl start docker
  echo "Docker CE installed successfully."
else
  echo "Docker CE and Buildx are already installed."
fi

# Install kind
if ! check_command kind || ! kind --version >/dev/null 2>&1; then
  echo "Installing kind $KIND_VERSION for arch $KIND_ARCH..."
  curl -Lo ./kind "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-linux-${KIND_ARCH}"
  chmod +x ./kind
  sudo install ./kind /usr/local/bin/kind
  rm ./kind
  echo "kind installed successfully."
else
  echo "kind is already installed: $(kind --version)"
fi

# Install kubectl
if ! check_command kubectl || ! kubectl version --client >/dev/null 2>&1; then
  echo "Installing kubectl for arch $KUBE_ARCH..."
  curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/${KUBE_ARCH}/kubectl"
  sudo install kubectl /usr/local/bin/kubectl
  rm kubectl
  echo "kubectl installed successfully."
else
  echo "kubectl is already installed and runnable."
fi

# Install Helm
if ! check_command helm; then
  echo "Helm not found. Installing Helm..."
  curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
  echo "Helm installed successfully."
else
  echo "Helm is already installed."
fi

# netcat is used by expose-cacheserver.sh to poll port-forward readiness.
if ! check_command nc; then
  echo "Installing netcat (nc)..."
  sudo apt-get update
  sudo apt-get install -y netcat-openbsd
else
  echo "nc is already installed."
fi
