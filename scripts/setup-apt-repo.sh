#!/usr/bin/env bash
# Setup script for initializing the GitHub Pages APT repository.
# Run once to: generate GPG key, create gh-pages branch with reprepro config, push.
#
# Prerequisites: git, gpg, gh (GitHub CLI, authenticated)
#
# Usage: bash scripts/setup-apt-repo.sh

set -euo pipefail

REPO_NAME="wpe-webkit-kiosk"
GPG_NAME="WPE Kiosk APT Repo"
GPG_EMAIL="apt@wpe-kiosk.local"
DIST_CODENAME="stable"
COMPONENT="main"
ARCH="amd64"

echo "=== 1. Generate GPG key ==="

GPG_PASSPHRASE=$(openssl rand -base64 32)
echo "Generated passphrase (save this!): $GPG_PASSPHRASE"

gpg --batch --gen-key <<EOF
%no-protection
Key-Type: RSA
Key-Length: 4096
Subkey-Type: RSA
Subkey-Length: 4096
Name-Real: ${GPG_NAME}
Name-Email: ${GPG_EMAIL}
Expire-Date: 0
Passphrase: ${GPG_PASSPHRASE}
%commit
EOF

GPG_KEY_ID=$(gpg --list-keys --with-colons "${GPG_EMAIL}" | grep '^pub' | head -1 | cut -d: -f5)
echo "Key ID: ${GPG_KEY_ID}"

echo ""
echo "=== 2. Export keys ==="

GPG_PRIVATE_KEY=$(gpg --batch --pinentry-mode loopback --passphrase "${GPG_PASSPHRASE}" \
  --armor --export-secret-keys "${GPG_KEY_ID}")

TMPDIR=$(mktemp -d)
gpg --armor --export "${GPG_KEY_ID}" > "${TMPDIR}/gpg.key"
echo "Public key exported to ${TMPDIR}/gpg.key"

echo ""
echo "=== 3. Create gh-pages branch ==="

WORK=$(mktemp -d)
git clone --no-checkout "$(git remote get-url origin)" "${WORK}"
cd "${WORK}"
git checkout --orphan gh-pages
git rm -rf . 2>/dev/null || true

# reprepro configuration
mkdir -p conf
cat > conf/distributions <<DISTEOF
Origin: ${GPG_NAME}
Label: ${REPO_NAME}
Codename: ${DIST_CODENAME}
Architectures: ${ARCH}
Components: ${COMPONENT}
Description: APT repository for ${REPO_NAME}
SignWith: ${GPG_KEY_ID}
DISTEOF

cp "${TMPDIR}/gpg.key" gpg.key

git add -A
git commit -m "chore: initialize gh-pages APT repository"
git push origin gh-pages

cd -
rm -rf "${WORK}" "${TMPDIR}"

echo ""
echo "=== 4. Add GitHub secrets ==="
echo ""
echo "Run the following commands to add secrets (requires gh CLI):"
echo ""
echo "  gh secret set GPG_PRIVATE_KEY <<'SECRETEOF'"
echo "${GPG_PRIVATE_KEY}" | head -2
echo "  ... (truncated)"
echo "  SECRETEOF"
echo ""
echo "  gh secret set GPG_PASSPHRASE --body '${GPG_PASSPHRASE}'"
echo ""
echo "Or add them manually at: https://github.com/tomasz-mizak/${REPO_NAME}/settings/secrets/actions"
echo ""

# Offer to set secrets automatically
read -rp "Set secrets via gh CLI now? [y/N] " answer
if [[ "${answer}" =~ ^[Yy]$ ]]; then
  echo "${GPG_PRIVATE_KEY}" | gh secret set GPG_PRIVATE_KEY
  gh secret set GPG_PASSPHRASE --body "${GPG_PASSPHRASE}"
  echo "Secrets set!"
fi

echo ""
echo "=== 5. Enable GitHub Pages ==="
echo ""
echo "Go to: https://github.com/tomasz-mizak/${REPO_NAME}/settings/pages"
echo "  Source: Deploy from a branch"
echo "  Branch: gh-pages / root"
echo ""
echo "Done! The next tagged release will publish to the APT repo."
