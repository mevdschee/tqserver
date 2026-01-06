#!/bin/bash
set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

if [ $# -lt 1 ]; then
    echo "Usage: $0 <target> [workers...]"
    echo ""
    echo "Targets: staging, production, custom"
    echo ""
    echo "Examples:"
    echo "  $0 production              # Deploy all workers and server"
    echo "  $0 production index        # Deploy only index worker"
    echo "  $0 production index api    # Deploy index and api workers"
    echo "  $0 staging                 # Deploy to staging environment"
    echo "  $0 custom user@host:/path  # Deploy to custom location"
    exit 1
fi

TARGET=$1
shift
DEPLOY_WORKERS=("$@")

# Load deployment config
case $TARGET in
    staging)
        SERVER="deploy@staging.example.com"
        DEPLOY_PATH="/opt/tqserver"
        ;;
    production)
        SERVER="deploy@prod.example.com"
        DEPLOY_PATH="/opt/tqserver"
        ;;
    custom)
        if [ $# -lt 1 ]; then
            echo -e "${RED}Error: Custom target requires server:path${NC}"
            echo "Example: $0 custom user@host:/opt/tqserver"
            exit 1
        fi
        SERVER_PATH=$1
        SERVER="${SERVER_PATH%%:*}"
        DEPLOY_PATH="${SERVER_PATH#*:}"
        shift
        DEPLOY_WORKERS=("$@")
        ;;
    *)
        echo -e "${RED}Unknown target: $TARGET${NC}"
        exit 1
        ;;
esac

echo -e "${GREEN}Deploying to $TARGET...${NC}"
echo "Server: $SERVER"
echo "Path: $DEPLOY_PATH"
echo ""

# Check if server is reachable
if ! ssh -o ConnectTimeout=5 "$SERVER" "echo 'Connection successful'" >/dev/null 2>&1; then
    echo -e "${RED}Error: Cannot connect to $SERVER${NC}"
    exit 1
fi

# Ensure remote directory exists
ssh "$SERVER" "mkdir -p $DEPLOY_PATH/{server/{bin,public,private},workers,logs,config}"

# Function to deploy server
deploy_server() {
    echo -e "${YELLOW}Deploying server...${NC}"
    
    # Deploy server binary
    if [ -f "server/bin/tqserver" ]; then
        rsync -avz --checksum server/bin/tqserver "$SERVER:$DEPLOY_PATH/server/bin/"
        echo "  ✓ Server binary deployed"
    else
        echo -e "${RED}  ✗ Server binary not found. Run: bash scripts/build-prod.sh${NC}"
        return 1
    fi
    
    # Deploy server assets
    if [ -d "server/public" ] && [ "$(ls -A server/public 2>/dev/null)" ]; then
        rsync -avz --checksum server/public/ "$SERVER:$DEPLOY_PATH/server/public/"
        echo "  ✓ Server public assets deployed"
    fi
    
    if [ -d "server/private" ] && [ "$(ls -A server/private 2>/dev/null)" ]; then
        rsync -avz --checksum server/private/ "$SERVER:$DEPLOY_PATH/server/private/"
        echo "  ✓ Server private resources deployed"
    fi
}

# Function to deploy a worker
deploy_worker() {
    local worker=$1
    echo -e "${YELLOW}Deploying worker: $worker...${NC}"
    
    if [ ! -d "workers/$worker" ]; then
        echo -e "${RED}  ✗ Worker directory not found: workers/$worker${NC}"
        return 1
    fi
    
    # Check if binary exists
    if [ ! -f "workers/$worker/bin/$worker" ]; then
        echo -e "${RED}  ✗ Worker binary not found: workers/$worker/bin/$worker${NC}"
        echo "     Run: bash scripts/build-prod.sh"
        return 1
    fi
    
    # Deploy entire worker directory (excluding src)
    rsync -avz --checksum --exclude='src' "workers/$worker/" "$SERVER:$DEPLOY_PATH/workers/$worker/"
    echo "  ✓ Worker $worker deployed"
}

# If no specific workers specified, deploy all
if [ ${#DEPLOY_WORKERS[@]} -eq 0 ]; then
    echo -e "${GREEN}Deploying: all workers and server${NC}"
    echo ""
    
    # Deploy server
    deploy_server
    echo ""
    
    # Deploy all workers
    echo -e "${YELLOW}Deploying all workers...${NC}"
    for worker_dir in workers/*/; do
        if [ -d "$worker_dir" ]; then
            worker=$(basename "$worker_dir")
            deploy_worker "$worker"
        fi
    done
else
    # Deploy specific workers only
    echo -e "${GREEN}Deploying: ${DEPLOY_WORKERS[*]}${NC}"
    echo ""
    
    for worker in "${DEPLOY_WORKERS[@]}"; do
        deploy_worker "$worker"
    done
fi

# Deploy config if it exists
if [ -f "config/server.yaml" ]; then
    echo ""
    echo -e "${YELLOW}Deploying configuration...${NC}"
    rsync -avz --checksum config/server.yaml "$SERVER:$DEPLOY_PATH/config/"
    echo "  ✓ Configuration deployed"
fi

echo ""
echo -e "${GREEN}✅ Deployed to $TARGET successfully${NC}"
echo ""
echo "Next steps:"
echo -e "  1. Trigger reload: ${YELLOW}ssh $SERVER 'killall -HUP tqserver'${NC}"
echo -e "  2. Monitor logs:   ${YELLOW}ssh $SERVER 'tail -f $DEPLOY_PATH/logs/server_*.log'${NC}"
echo -e "  3. Check status:   ${YELLOW}ssh $SERVER 'systemctl status tqserver'${NC}"
