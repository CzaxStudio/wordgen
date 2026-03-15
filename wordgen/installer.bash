#!/usr/bin/env bash

# =============================================================================
#  WordGen Installer
#  Supports: Linux (amd64/arm64), macOS (Intel/Apple Silicon)
#  Run as root or with sudo for system-wide installation
# =============================================================================

set -euo pipefail

# ─── Configuration ────────────────────────────────────────────────────────────

TOOL_NAME="wordgen"
VERSION="1.0.0"
REPO="https://github.com/yourusername/wordgen"
INSTALL_DIR="/usr/local/bin"
TMP_DIR="$(mktemp -d)"
LOG_FILE="/tmp/wordgen-install.log"

# ─── Colors ───────────────────────────────────────────────────────────────────

RESET="\033[0m"
BOLD="\033[1m"
DIM="\033[2m"
RED="\033[31m"
GREEN="\033[32m"
YELLOW="\033[33m"
CYAN="\033[36m"
WHITE="\033[97m"

# ─── Helpers ──────────────────────────────────────────────────────────────────

print_banner() {
    echo -e "${CYAN}${BOLD}"
    echo "  ██╗    ██╗ ██████╗ ██████╗ ██████╗  ██████╗ ███████╗███╗   ██╗"
    echo "  ██║    ██║██╔═══██╗██╔══██╗██╔══██╗██╔════╝ ██╔════╝████╗  ██║"
    echo "  ██║ █╗ ██║██║   ██║██████╔╝██║  ██║██║  ███╗█████╗  ██╔██╗ ██║"
    echo "  ██║███╗██║██║   ██║██╔══██╗██║  ██║██║   ██║██╔══╝  ██║╚██╗██║"
    echo "  ╚███╔███╔╝╚██████╔╝██║  ██║██████╔╝╚██████╔╝███████╗██║ ╚████║"
    echo "   ╚══╝╚══╝  ╚═════╝ ╚═╝  ╚═╝╚═════╝  ╚═════╝ ╚══════╝╚═╝  ╚═══╝"
    echo -e "${RESET}"
    echo -e "${DIM}  Professional Wordlist Generator — Installer v${VERSION}${RESET}"
    echo -e "${DIM}  ──────────────────────────────────────────────────────${RESET}"
    echo ""
}

info()    { echo -e "  ${CYAN}[*]${RESET} $*"; }
success() { echo -e "  ${GREEN}[+]${RESET} $*"; }
warn()    { echo -e "  ${YELLOW}[!]${RESET} $*"; }
error()   { echo -e "  ${RED}[x]${RESET} $*" >&2; }
die()     { error "$*"; cleanup; exit 1; }

step() {
    echo ""
    echo -e "  ${BOLD}${WHITE}$*${RESET}"
    echo -e "  ${DIM}$(printf '%.0s─' {1..50})${RESET}"
}

cleanup() {
    rm -rf "${TMP_DIR}"
}

trap cleanup EXIT

# ─── Root / Sudo Check ────────────────────────────────────────────────────────

check_privileges() {
    if [[ "${EUID}" -ne 0 ]]; then
        warn "Not running as root. Attempting sudo..."
        exec sudo bash "$0" "$@"
    fi
    success "Running with root privileges."
}

# ─── OS / Architecture Detection ─────────────────────────────────────────────

detect_platform() {
    step "Detecting platform"

    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "${OS}" in
        Linux)
            PLATFORM="linux"
            ;;
        Darwin)
            PLATFORM="darwin"
            ;;
        *)
            die "Unsupported OS: ${OS}. Only Linux and macOS are supported."
            ;;
    esac

    case "${ARCH}" in
        x86_64)
            GOARCH="amd64"
            ;;
        aarch64 | arm64)
            GOARCH="arm64"
            ;;
        *)
            die "Unsupported architecture: ${ARCH}."
            ;;
    esac

    info "OS        : ${OS}"
    info "Arch      : ${ARCH}"
    info "Platform  : ${PLATFORM}-${GOARCH}"
}

# ─── Dependency Check ─────────────────────────────────────────────────────────

check_dependencies() {
    step "Checking dependencies"

    local deps=("curl" "chmod" "install")
    local missing=()

    for dep in "${deps[@]}"; do
        if command -v "${dep}" &>/dev/null; then
            success "${dep} found"
        else
            missing+=("${dep}")
        fi
    done

    if [[ ${#missing[@]} -gt 0 ]]; then
        warn "Missing dependencies: ${missing[*]}"
        info "Attempting to install missing packages..."

        if command -v apt-get &>/dev/null; then
            apt-get update -qq && apt-get install -y -qq "${missing[@]}" \
                >> "${LOG_FILE}" 2>&1 || die "Failed to install dependencies."
        elif command -v yum &>/dev/null; then
            yum install -y -q "${missing[@]}" \
                >> "${LOG_FILE}" 2>&1 || die "Failed to install dependencies."
        elif command -v brew &>/dev/null; then
            brew install "${missing[@]}" \
                >> "${LOG_FILE}" 2>&1 || die "Failed to install dependencies."
        else
            die "Cannot auto-install dependencies. Please install manually: ${missing[*]}"
        fi

        success "Dependencies installed."
    fi
}

# ─── Download Binary ──────────────────────────────────────────────────────────

download_binary() {
    step "Downloading WordGen"

    # Determine binary name matching your release assets
    if [[ "${PLATFORM}" == "darwin" && "${GOARCH}" == "arm64" ]]; then
        BINARY_NAME="${TOOL_NAME}-macos-arm64"
    elif [[ "${PLATFORM}" == "darwin" && "${GOARCH}" == "amd64" ]]; then
        BINARY_NAME="${TOOL_NAME}-macos-intel"
    elif [[ "${PLATFORM}" == "linux" && "${GOARCH}" == "amd64" ]]; then
        BINARY_NAME="${TOOL_NAME}-linux-amd64"
    elif [[ "${PLATFORM}" == "linux" && "${GOARCH}" == "arm64" ]]; then
        BINARY_NAME="${TOOL_NAME}-linux-arm64"
    else
        die "No prebuilt binary for ${PLATFORM}-${GOARCH}."
    fi

    DOWNLOAD_URL="${REPO}/releases/download/v${VERSION}/${BINARY_NAME}"
    DEST="${TMP_DIR}/${TOOL_NAME}"

    info "Binary    : ${BINARY_NAME}"
    info "Source    : ${DOWNLOAD_URL}"
    info "Temp path : ${DEST}"
    echo ""

    if ! curl -fSL --progress-bar "${DOWNLOAD_URL}" -o "${DEST}" 2>&1; then
        warn "Remote download failed. Checking for local build..."
        build_from_source
    else
        success "Download complete."
    fi
}

# ─── Fallback: Build from Source ─────────────────────────────────────────────

build_from_source() {
    info "Attempting local build from source..."

    if ! command -v go &>/dev/null; then
        die "Go is not installed and remote binary unavailable. Install Go from https://golang.org/dl/ or check your internet connection."
    fi

    GO_VERSION="$(go version | awk '{print $3}' | sed 's/go//')"
    info "Go found  : ${GO_VERSION}"

    if [[ -f "main.go" ]]; then
        info "Building from current directory..."
        go build -o "${TMP_DIR}/${TOOL_NAME}" . >> "${LOG_FILE}" 2>&1 \
            || die "Build failed. Check ${LOG_FILE} for details."
        success "Built successfully from source."
    else
        die "No main.go found. Please run this installer from the project root or ensure internet access for binary download."
    fi
}

# ─── Install Binary ───────────────────────────────────────────────────────────

install_binary() {
    step "Installing binary"

    local src="${TMP_DIR}/${TOOL_NAME}"
    local dest="${INSTALL_DIR}/${TOOL_NAME}"

    # Make sure install dir exists
    mkdir -p "${INSTALL_DIR}"

    # Remove old version if exists
    if [[ -f "${dest}" ]]; then
        warn "Existing installation found at ${dest}. Replacing..."
        rm -f "${dest}"
    fi

    chmod +x "${src}"
    install -m 0755 "${src}" "${dest}" \
        || die "Failed to install binary to ${dest}."

    success "Installed to ${dest}"
}

# ─── Verify Installation ──────────────────────────────────────────────────────

verify_installation() {
    step "Verifying installation"

    if command -v "${TOOL_NAME}" &>/dev/null; then
        INSTALLED_PATH="$(command -v ${TOOL_NAME})"
        success "Binary found at : ${INSTALLED_PATH}"
        success "Verification    : PASSED"
    else
        warn "${INSTALL_DIR} may not be in your PATH."
        warn "Add the following to your shell profile:"
        echo ""
        echo -e "  ${CYAN}export PATH=\"\$PATH:${INSTALL_DIR}\"${RESET}"
        echo ""
    fi
}

# ─── Shell Completion (Optional) ─────────────────────────────────────────────

setup_shell_hint() {
    step "Shell configuration"

    local profile=""

    if [[ -n "${BASH_VERSION:-}" ]]; then
        profile="${HOME}/.bashrc"
    elif [[ -n "${ZSH_VERSION:-}" ]]; then
        profile="${HOME}/.zshrc"
    fi

    if [[ -n "${profile}" && -f "${profile}" ]]; then
        if ! grep -q "wordgen" "${profile}" 2>/dev/null; then
            echo "" >> "${profile}"
            echo "# WordGen — added by installer" >> "${profile}"
            echo "export PATH=\"\$PATH:${INSTALL_DIR}\"" >> "${profile}"
            success "PATH updated in ${profile}"
            info "Run: source ${profile}"
        else
            info "PATH already configured in ${profile}"
        fi
    fi
}

# ─── Uninstall Mode ───────────────────────────────────────────────────────────

uninstall() {
    step "Uninstalling WordGen"

    local dest="${INSTALL_DIR}/${TOOL_NAME}"

    if [[ -f "${dest}" ]]; then
        rm -f "${dest}"
        success "Removed ${dest}"
    else
        warn "WordGen is not installed at ${dest}."
    fi

    exit 0
}

# ─── Print Summary ────────────────────────────────────────────────────────────

print_summary() {
    echo ""
    echo -e "  ${GREEN}${BOLD}Installation complete.${RESET}"
    echo ""
    echo -e "  ${DIM}──────────────────────────────────────────────────────${RESET}"
    echo -e "  ${BOLD}Quick start:${RESET}"
    echo ""
    echo -e "  ${CYAN}wordgen -mode=keyword -keywords=\"admin,root\" -leet -cap -years${RESET}"
    echo -e "  ${CYAN}wordgen -mode=brute -charset=digits -min=4 -max=4${RESET}"
    echo -e "  ${CYAN}wordgen -mode=pattern -pattern=admin##${RESET}"
    echo ""
    echo -e "  ${DIM}For full usage: wordgen --help${RESET}"
    echo -e "  ${DIM}Repo          : ${REPO}${RESET}"
    echo -e "  ${DIM}Log file      : ${LOG_FILE}${RESET}"
    echo -e "  ${DIM}──────────────────────────────────────────────────────${RESET}"
    echo ""
    echo -e "  ${YELLOW}For authorized penetration testing use only.${RESET}"
    echo ""
}

# ─── Argument Parsing ─────────────────────────────────────────────────────────

parse_args() {
    for arg in "$@"; do
        case "${arg}" in
            --uninstall)
                check_privileges "$@"
                uninstall
                ;;
            --dir=*)
                INSTALL_DIR="${arg#*=}"
                ;;
            --version=*)
                VERSION="${arg#*=}"
                ;;
            --help | -h)
                echo ""
                echo -e "  ${BOLD}WordGen Installer${RESET}"
                echo ""
                echo "  Usage: bash installer.bash [options]"
                echo ""
                echo "  Options:"
                echo "    --uninstall        Remove WordGen from the system"
                echo "    --dir=PATH         Install to a custom directory (default: /usr/local/bin)"
                echo "    --version=X.Y.Z    Install a specific version (default: ${VERSION})"
                echo "    --help             Show this help message"
                echo ""
                exit 0
                ;;
        esac
    done
}

# ─── Entry Point ──────────────────────────────────────────────────────────────

main() {
    print_banner
    parse_args "$@"
    check_privileges "$@"
    detect_platform
    check_dependencies
    download_binary
    install_binary
    verify_installation
    setup_shell_hint
    print_summary
}

main "$@"