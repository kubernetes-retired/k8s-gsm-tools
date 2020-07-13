FROM golang:1.13.12

WORKDIR /workspace

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
RUN go mod download


# install kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
    chmod +x ./kubectl && \
    mv ./kubectl /usr/local/bin/kubectl

# install gcloud
ENV PATH=/google-cloud-sdk/bin:/workspace:${PATH} \
    CLOUDSDK_CORE_DISABLE_PROMPTS=1


RUN wget -q https://dl.google.com/dl/cloudsdk/channels/rapid/google-cloud-sdk.tar.gz && \
    tar xzf google-cloud-sdk.tar.gz -C / && \
    rm google-cloud-sdk.tar.gz && \
    /google-cloud-sdk/install.sh \
        --disable-installation-options \
        --bash-completion=false \
        --path-update=false \
        --usage-reporting=false && \
    gcloud info | tee /workspace/gcloud-info.txt

COPY . .
