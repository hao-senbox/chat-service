#!/usr/bin/env bash
# Script to wait for Consul to be ready

CONSUL_cmdname=${0##*/}

echoerr() { 
    if [[ $CONSUL_QUIET -ne 1 ]]; then 
        echo "$@" 1>&2
    fi 
}

usage()
{
    cat << USAGE >&2
Usage:
    $CONSUL_cmdname host:port [-s] [-t timeout] [-- command args]
    -h HOST | --host=HOST       Consul host or IP
    -p PORT | --port=PORT       Consul port (default: 8500)
                                Alternatively, you specify the host and port as host:port
    -s | --strict               Only execute subcommand if consul is ready
    -q | --quiet                Don't output any status messages
    -t TIMEOUT | --timeout=TIMEOUT
                                Timeout in seconds (default: 30)
    -- COMMAND ARGS             Execute command with args after consul is ready
USAGE
    exit 1
}

# Check if Consul is ready by testing API endpoints
check_consul_ready()
{
    local host=$1
    local port=$2
    
    # First check basic TCP connectivity
    if ! (echo -n > /dev/tcp/$host/$port) >/dev/null 2>&1; then
        return 1
    fi
    
    # Check if curl is available
    if command -v curl >/dev/null 2>&1; then
        # Check Consul leader election (means cluster is ready)
        local leader=$(curl -sf "http://$host:$port/v1/status/leader" 2>/dev/null)
        if [[ $? -eq 0 && "$leader" != '""' && "$leader" != "" ]]; then
            # Also check agent status
            curl -sf "http://$host:$port/v1/agent/self" >/dev/null 2>&1
            return $?
        fi
    elif command -v wget >/dev/null 2>&1; then
        # Fallback to wget
        local leader=$(wget -qO- "http://$host:$port/v1/status/leader" 2>/dev/null)
        if [[ $? -eq 0 && "$leader" != '""' && "$leader" != "" ]]; then
            wget -qO- "http://$host:$port/v1/agent/self" >/dev/null 2>&1
            return $?
        fi
    else
        # If no HTTP client available, just check TCP
        return 0
    fi
    
    return 1
}

wait_for_consul()
{
    if [[ $CONSUL_TIMEOUT -gt 0 ]]; then
        echoerr "$CONSUL_cmdname: waiting $CONSUL_TIMEOUT seconds for Consul at $CONSUL_HOST:$CONSUL_PORT"
    else
        echoerr "$CONSUL_cmdname: waiting for Consul at $CONSUL_HOST:$CONSUL_PORT without a timeout"
    fi
    
    CONSUL_start_ts=$(date +%s)
    while :
    do
        if check_consul_ready $CONSUL_HOST $CONSUL_PORT; then
            CONSUL_end_ts=$(date +%s)
            echoerr "$CONSUL_cmdname: Consul at $CONSUL_HOST:$CONSUL_PORT is ready after $((CONSUL_end_ts - CONSUL_start_ts)) seconds"
            break
        fi
        
        # Check timeout
        if [[ $CONSUL_TIMEOUT -gt 0 ]]; then
            CONSUL_current_ts=$(date +%s)
            if [[ $((CONSUL_current_ts - CONSUL_start_ts)) -ge $CONSUL_TIMEOUT ]]; then
                echoerr "$CONSUL_cmdname: timeout occurred after waiting $CONSUL_TIMEOUT seconds for Consul at $CONSUL_HOST:$CONSUL_PORT"
                return 1
            fi
        fi
        
        sleep 2
    done
    return 0
}

# Process arguments (similar to wait-for-it.sh structure)
while [[ $# -gt 0 ]]
do
    case "$1" in
        *:* )
        CONSUL_hostport=(${1//:/ })
        CONSUL_HOST=${CONSUL_hostport[0]}
        CONSUL_PORT=${CONSUL_hostport[1]}
        shift 1
        ;;
        -q | --quiet)
        CONSUL_QUIET=1
        shift 1
        ;;
        -s | --strict)
        CONSUL_STRICT=1
        shift 1
        ;;
        -h)
        CONSUL_HOST="$2"
        if [[ $CONSUL_HOST == "" ]]; then break; fi
        shift 2
        ;;
        --host=*)
        CONSUL_HOST="${1#*=}"
        shift 1
        ;;
        -p)
        CONSUL_PORT="$2"
        if [[ $CONSUL_PORT == "" ]]; then break; fi
        shift 2
        ;;
        --port=*)
        CONSUL_PORT="${1#*=}"
        shift 1
        ;;
        -t)
        CONSUL_TIMEOUT="$2"
        if [[ $CONSUL_TIMEOUT == "" ]]; then break; fi
        shift 2
        ;;
        --timeout=*)
        CONSUL_TIMEOUT="${1#*=}"
        shift 1
        ;;
        --)
        shift
        CONSUL_CLI=("$@")
        break
        ;;
        --help)
        usage
        ;;
        *)
        echoerr "Unknown argument: $1"
        usage
        ;;
    esac
done

if [[ "$CONSUL_HOST" == "" || "$CONSUL_PORT" == "" ]]; then
    echoerr "Error: you need to provide a host and port to test."
    usage
fi

# Set defaults
CONSUL_TIMEOUT=${CONSUL_TIMEOUT:-30}
CONSUL_STRICT=${CONSUL_STRICT:-0}
CONSUL_QUIET=${CONSUL_QUIET:-0}

# Wait for Consul
wait_for_consul
CONSUL_RESULT=$?

# Execute command if provided
if [[ $CONSUL_CLI != "" ]]; then
    if [[ $CONSUL_RESULT -ne 0 && $CONSUL_STRICT -eq 1 ]]; then
        echoerr "$CONSUL_cmdname: strict mode, refusing to execute subprocess"
        exit $CONSUL_RESULT
    fi
    exec "${CONSUL_CLI[@]}"
else
    exit $CONSUL_RESULT
fi