#!/bin/bash
set -e

cd "$(dirname "$0")/.."

# Setup Node + nvm

sudo chown -R vscode:vscode $PROJECT_PATH/node_modules

# Install the Node version specified in .nvmrc
export NVM_DIR="/usr/local/share/nvm"
# shellcheck source=/dev/null
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
if [ -f .nvmrc ]; then
    nvm install
fi


# Install frontend dependencies
if [ -f frontend/package.json ]; then
    cd frontend && npm install && cd ..
fi


# Setup agents
sudo chown -R vscode:vscode /home/vscode/.claude

curl -fsSL https://claude.ai/install.sh | bash

curl -fsSL https://gh.io/copilot-install | bash


# Setup bash

# Setup custom prompt - hybrid of local + container features
cat >> ~/.bashrc << 'EOF'

# Automatically use the correct Node version when entering the project
export NVM_DIR="/usr/local/share/nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
cd() { builtin cd "$@" && [ -f .nvmrc ] && nvm use --silent; }

# Auto-switch to .nvmrc version on shell start
[ -f .nvmrc ] && nvm use --silent

cls ()
{
    clear && printf '\033[3J'
}

run_task_loop()
{
    $PROJECT_PATH/.claude/skills/task-tracking/scripts/run_task_loop.sh "$@";
}

# Custom prompt - hybrid of local + container features
export PS1='\[\]`export XIT=$?; [ "$XIT" -ne 0 ] && echo -n "\[\033[1;31m\]" || echo -n "\[\033[0m\]"`container`export FOLDER=$(basename "$PWD"); export BRANCH="$(git --no-optional-locks symbolic-ref --short HEAD 2>/dev/null || git --no-optional-locks rev-parse --short HEAD 2>/dev/null)"; if [ "${BRANCH:-}" != "" ]; then [ "$FOLDER" != "$BRANCH" ] && echo -n " \[\033[32m\]$FOLDER"; echo -n " \[\033[33m\]($BRANCH)"; else echo -n " \[\033[32m\]$FOLDER"; fi`\[\033[00m\] $ \[\]'
EOF

