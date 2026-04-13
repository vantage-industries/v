FROM ubuntu:24.04

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Prevent hash changing on bitbake
ENV PYTHONHASHSEED=0

# Set locale
ENV LANG=en_US.UTF-8
ENV LC_ALL=en_US.UTF-8

# Install Yocto build host packages per official documentation + tmux for bitbake
RUN apt-get update && apt-get install -y \
    build-essential \
    chrpath \
    cpio \
    debianutils \
    diffstat \
    file \
    gawk \
    gcc \
    git \
    iproute2 \
    iputils-ping \
    libacl1 \
    locales \
    python3 \
    python3-git \
    python3-jinja2 \
    python3-pexpect \
    python3-pip \
    python3-subunit \
    socat \
    texinfo \
    unzip \
    wget \
    xz-utils \
    zstd \
    rpcbind \
    sudo \
    tmux \
    && rm -rf /var/lib/apt/lists/*

# Configure locale
RUN locale-gen en_US.UTF-8

# Create builder user with host UID/GID
ARG USER_ID=1000
ARG GROUP_ID=1000
ARG USER_NAME=builder

RUN \
    # Handle existing group or create new one
    if ! getent group $GROUP_ID > /dev/null 2>&1; then \
        groupadd -g $GROUP_ID $USER_NAME; \
    fi && \
    # Handle existing user or create new one
    if ! getent passwd $USER_ID > /dev/null 2>&1; then \
        useradd -m -u $USER_ID -g $GROUP_ID -s /bin/bash $USER_NAME; \
    else \
        # If UID exists, rename the existing user to our name
        EXISTING_USER=$(getent passwd $USER_ID | cut -d: -f1) && \
        usermod -l $USER_NAME $EXISTING_USER && \
        usermod -d /home/$USER_NAME -m $USER_NAME; \
    fi && \
    usermod -aG sudo $USER_NAME && \
    echo "$USER_NAME ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Switch to builder user
USER $USER_NAME
WORKDIR /home/$USER_NAME

# Set up environment
ENV HOME=/home/$USER_NAME
ENV BB_NUMBER_THREADS=8
ENV PARALLEL_MAKE="-j8"

# Increase file descriptor limits for Yocto builds
RUN echo "ulimit -n 4096" >> ~/.bashrc

# Default command
CMD ["/bin/bash"]
