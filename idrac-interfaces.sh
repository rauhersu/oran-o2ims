#!/bin/bash
#
# idrac-interfaces.sh - List network interfaces from iDRAC with guessed Linux names
#
# Usage: ./idrac-interfaces.sh <idrac_ip> -u <username> -p <password>
#

set -e

# Credentials (no defaults for security)
IDRAC_IP=""
USERNAME=""
PASSWORD=""

# Function to show usage
usage() {
    echo "Usage: $0 -u <username> -p <password> <idrac_ip>"
    echo ""
    echo "Arguments:"
    echo "  -u <username>     iDRAC username (required)"
    echo "  -p <password>     iDRAC password (required)"
    echo "  <idrac_ip>        iDRAC IP address or hostname (required, last argument)"
    echo "  -h                Show this help message"
    echo ""
    echo "Example: $0 -u admin -p mypassword 10.8.35.49"
    exit 1
}

# Parse arguments
if [[ $# -lt 1 || "$1" == "-h" || "$1" == "--help" ]]; then
    usage
fi

# Parse flags first, IP address is the last argument
while [[ $# -gt 1 ]]; do
    case "$1" in
        -u)
            USERNAME="$2"
            shift 2
            ;;
        -p)
            PASSWORD="$2"
            shift 2
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo "ERROR: Unknown option: $1"
            usage
            ;;
    esac
done

# Last argument is the IP address
IDRAC_IP="$1"

if [[ -z "$USERNAME" ]]; then
    echo "ERROR: Username is required (-u)"
    usage
fi

if [[ -z "$PASSWORD" ]]; then
    echo "ERROR: Password is required (-p)"
    usage
fi

if [[ -z "$IDRAC_IP" ]]; then
    echo "ERROR: iDRAC IP address is required (last argument)"
    usage
fi

BASE_URL="https://${IDRAC_IP}/redfish/v1/Systems/System.Embedded.1/EthernetInterfaces"
CHASSIS_URL="https://${IDRAC_IP}/redfish/v1/Chassis/System.Embedded.1/NetworkAdapters"

# Curl options
CURL_OPTS="-L -ks -u ${USERNAME}:${PASSWORD}"

# Cache for OUI lookups (to avoid repeated API calls)
declare -A OUI_CACHE

# Function to lookup manufacturer from MAC OUI via macvendors.com API
lookup_oui() {
    local mac="$1"
    local oui="${mac:0:8}"  # First 8 chars (AA:BB:CC)

    # Check cache first
    if [[ -n "${OUI_CACHE[$oui]}" ]]; then
        echo "${OUI_CACHE[$oui]}"
        return
    fi

    # Query the macvendors.com API
    local vendor
    vendor=$(curl -s --max-time 2 "https://api.macvendors.com/${oui}" 2>/dev/null)

    # Check if we got a valid response (not an error message)
    if [[ -n "$vendor" && ! "$vendor" =~ "Not Found" && ! "$vendor" =~ "errors" ]]; then
        # Shorten common vendor names for display
        vendor=$(echo "$vendor" | sed -E '
            s/Mellanox Technologies, Inc\./Mellanox/;
            s/Intel Corporate/Intel/;
            s/Intel Corporation/Intel/;
            s/Dell Inc\./Dell/;
            s/Broadcom Inc\. and subsidiaries/Broadcom/;
            s/Broadcom Limited/Broadcom/;
            s/NVIDIA Corporation/NVIDIA/;
            s/Cisco Systems, Inc/Cisco/;
            s/Hewlett Packard Enterprise/HPE/;
            s/Hewlett Packard/HP/;
        ')
        # Truncate to first word if still too long
        if [[ ${#vendor} -gt 15 ]]; then
            vendor=$(echo "$vendor" | awk '{print $1}')
        fi
    else
        vendor="Unknown"
    fi

    # Cache the result
    OUI_CACHE[$oui]="$vendor"

    echo "$vendor"
}

# Function to guess Linux interface name from iDRAC ID
guess_linux_name() {
    local idrac_id="$1"

    # Pattern: NIC.Integrated.<adapter>-<port>-<partition>
    if [[ "$idrac_id" =~ ^NIC\.Integrated\.([0-9]+)-([0-9]+)-([0-9]+)$ ]]; then
        local port="${BASH_REMATCH[2]}"
        echo "eno${port}"
        return
    fi

    # Pattern: NIC.Slot.<slot>-<port>-<partition>
    if [[ "$idrac_id" =~ ^NIC\.Slot\.([0-9]+)-([0-9]+)-([0-9]+)$ ]]; then
        local slot="${BASH_REMATCH[1]}"
        local port="${BASH_REMATCH[2]}"
        echo "ens${slot}f$((port-1))"
        return
    fi

    # Pattern: NIC.Embedded.<adapter>-<port>-<partition>
    if [[ "$idrac_id" =~ ^NIC\.Embedded\.([0-9]+)-([0-9]+)-([0-9]+)$ ]]; then
        local adapter="${BASH_REMATCH[1]}"
        local port="${BASH_REMATCH[2]}"
        echo "em${adapter}"
        return
    fi

    echo "unknown"
}

# Function to get adapter base name from interface ID (e.g., NIC.Slot.6-1-1 -> NIC.Slot.6)
get_adapter_id() {
    local iface_id="$1"

    if [[ "$iface_id" =~ ^(NIC\.[^.]+\.[0-9]+) ]]; then
        echo "${BASH_REMATCH[1]}"
    elif [[ "$iface_id" =~ ^(NIC\.[^.]+)\.[0-9]+-[0-9]+-[0-9]+$ ]]; then
        echo "${BASH_REMATCH[1]}"
    else
        echo ""
    fi
}

echo "============================================================"
echo "iDRAC Network Interfaces - ${IDRAC_IP}"
echo "============================================================"
echo ""

# First, get adapter details (model, speed, ports, sriov) from NetworkAdapters
declare -A ADAPTER_MODEL
declare -A ADAPTER_SPEED
declare -A ADAPTER_PORTS
declare -A ADAPTER_SRIOV

ADAPTERS=$(curl $CURL_OPTS "${CHASSIS_URL}" 2>/dev/null)
if [[ -n "$ADAPTERS" ]] && echo "$ADAPTERS" | jq -e . >/dev/null 2>&1; then
    ADAPTER_URIS=$(echo "$ADAPTERS" | jq -r '.Members[]."@odata.id"' 2>/dev/null)

    while IFS= read -r uri; do
        [[ -z "$uri" ]] && continue
        ADAPTER_DATA=$(curl $CURL_OPTS "https://${IDRAC_IP}${uri}" 2>/dev/null)

        if [[ -n "$ADAPTER_DATA" ]] && echo "$ADAPTER_DATA" | jq -e . >/dev/null 2>&1; then
            ADAPTER_ID=$(echo "$ADAPTER_DATA" | jq -r '.Id // ""')
            MODEL=$(echo "$ADAPTER_DATA" | jq -r '.Model // "N/A"')

            # Get port count and speed from Controllers
            PORT_COUNT=$(echo "$ADAPTER_DATA" | jq -r '.Controllers[0].ControllerCapabilities.NetworkPortCount // "N/A"')

            # Try to get speed from Ports
            PORTS_URI=$(echo "$ADAPTER_DATA" | jq -r '.Controllers[0].Links.NetworkPorts[0]."@odata.id" // empty')
            SPEED="N/A"
            if [[ -n "$PORTS_URI" ]]; then
                PORT_DATA=$(curl $CURL_OPTS "https://${IDRAC_IP}${PORTS_URI}" 2>/dev/null)
                if [[ -n "$PORT_DATA" ]] && echo "$PORT_DATA" | jq -e . >/dev/null 2>&1; then
                    SPEED=$(echo "$PORT_DATA" | jq -r '.CurrentLinkSpeedMbps // .SupportedLinkCapabilities[0].LinkSpeedMbps // "N/A"')
                    if [[ "$SPEED" != "N/A" && "$SPEED" != "null" ]]; then
                        # Convert to human readable
                        if [[ "$SPEED" -ge 100000 ]]; then
                            SPEED="$((SPEED/1000))GbE"
                        elif [[ "$SPEED" -ge 1000 ]]; then
                            SPEED="$((SPEED/1000))GbE"
                        else
                            SPEED="${SPEED}Mbps"
                        fi
                    fi
                fi
            fi

            # Get SR-IOV capability
            SRIOV_CAPABLE=$(echo "$ADAPTER_DATA" | jq -r '.Controllers[0].ControllerCapabilities.VirtualizationOffload.SRIOV.SRIOVVEPACapable // "null"')
            SRIOV_MAX_VFS=$(echo "$ADAPTER_DATA" | jq -r '.Controllers[0].ControllerCapabilities.VirtualizationOffload.VirtualFunction.DeviceMaxCount // "null"')

            if [[ "$SRIOV_CAPABLE" == "true" && "$SRIOV_MAX_VFS" != "null" && -n "$SRIOV_MAX_VFS" ]]; then
                SRIOV_INFO="Yes ($SRIOV_MAX_VFS)"
            elif [[ "$SRIOV_CAPABLE" == "true" ]]; then
                SRIOV_INFO="Yes"
            else
                SRIOV_INFO="No"
            fi

            # Store in associative arrays
            ADAPTER_MODEL["$ADAPTER_ID"]="$MODEL"
            ADAPTER_SPEED["$ADAPTER_ID"]="$SPEED"
            ADAPTER_PORTS["$ADAPTER_ID"]="$PORT_COUNT"
            ADAPTER_SRIOV["$ADAPTER_ID"]="$SRIOV_INFO"
        fi
    done <<< "$ADAPTER_URIS"
fi

# Get the list of interfaces
INTERFACES=$(curl $CURL_OPTS "${BASE_URL}" 2>/dev/null)

if [[ -z "$INTERFACES" ]] || ! echo "$INTERFACES" | jq -e . >/dev/null 2>&1; then
    echo "ERROR: Failed to connect to iDRAC or invalid response"
    echo "Check IP address and credentials"
    exit 1
fi

# Extract interface URIs
INTERFACE_URIS=$(echo "$INTERFACES" | jq -r '.Members[]."@odata.id"')

if [[ -z "$INTERFACE_URIS" ]]; then
    echo "No interfaces found"
    exit 0
fi

# Print header
printf "%-25s %-20s %-12s %-30s %-7s %-12s %-12s %-10s %-20s\n" \
    "iDRAC ID" "MAC Address" "Vendor" "Model" "Ports" "Speed" "SR-IOV" "Link" "OS Iface (guessed)"
printf "%-25s %-20s %-12s %-30s %-7s %-12s %-12s %-10s %-20s\n" \
    "-------------------------" "--------------------" "------------" "------------------------------" "-------" "------------" "------------" "----------" "--------------------"

# Iterate through each interface
while IFS= read -r uri; do
    # Get interface details
    IFACE_DATA=$(curl $CURL_OPTS "https://${IDRAC_IP}${uri}" 2>/dev/null)

    if [[ -z "$IFACE_DATA" ]] || ! echo "$IFACE_DATA" | jq -e . >/dev/null 2>&1; then
        continue
    fi

    # Extract fields
    IDRAC_ID=$(echo "$IFACE_DATA" | jq -r '.Id // "N/A"')
    MAC=$(echo "$IFACE_DATA" | jq -r '.MACAddress // "N/A"')
    LINK_STATUS=$(echo "$IFACE_DATA" | jq -r '.LinkStatus // "N/A"')
    IFACE_SPEED=$(echo "$IFACE_DATA" | jq -r '.SpeedMbps // "null"')

    # Lookup vendor from MAC OUI
    VENDOR=$(lookup_oui "$MAC")

    # Get adapter info
    # Derive adapter ID based on interface type
    # Note: For embedded NICs, Dell puts all ports under NIC.Embedded.1 adapter
    if [[ "$IDRAC_ID" =~ ^NIC\.Slot\.([0-9]+) ]]; then
        ADAPTER_KEY="NIC.Slot.${BASH_REMATCH[1]}"
    elif [[ "$IDRAC_ID" =~ ^NIC\.Integrated\.([0-9]+) ]]; then
        ADAPTER_KEY="NIC.Integrated.${BASH_REMATCH[1]}"
    elif [[ "$IDRAC_ID" =~ ^NIC\.Embedded\.[0-9]+-[0-9]+-[0-9]+$ ]]; then
        # For embedded NICs, always use NIC.Embedded.1 as the adapter key
        ADAPTER_KEY="NIC.Embedded.1"
    else
        ADAPTER_KEY=""
    fi

    MODEL="${ADAPTER_MODEL[$ADAPTER_KEY]:-N/A}"
    ADAPTER_SPEED_VAL="${ADAPTER_SPEED[$ADAPTER_KEY]:-N/A}"
    PORTS="${ADAPTER_PORTS[$ADAPTER_KEY]:-N/A}"
    SRIOV="${ADAPTER_SRIOV[$ADAPTER_KEY]:-N/A}"

    # If model is N/A or null, try to get it from Dell OEM NetworkDeviceFunction data
    if [[ "$MODEL" == "N/A" || "$MODEL" == "null" || -z "$MODEL" ]]; then
        # Build the NetworkDeviceFunction URL for this interface
        # e.g., /redfish/v1/Chassis/System.Embedded.1/NetworkAdapters/NIC.Embedded.1/NetworkDeviceFunctions/NIC.Embedded.1-1-1
        # Note: For embedded NICs, both ports may be under NIC.Embedded.1 adapter even if named NIC.Embedded.2-1-1
        OEM_ADAPTER_KEY="$ADAPTER_KEY"

        # For embedded NICs, Dell puts all ports under NIC.Embedded.1 adapter
        if [[ "$IDRAC_ID" =~ ^NIC\.Embedded\.[0-9]+-[0-9]+-[0-9]+$ ]]; then
            OEM_ADAPTER_KEY="NIC.Embedded.1"
        fi

        OEM_URL="https://${IDRAC_IP}/redfish/v1/Chassis/System.Embedded.1/NetworkAdapters/${OEM_ADAPTER_KEY}/NetworkDeviceFunctions/${IDRAC_ID}"
        OEM_DATA=$(curl $CURL_OPTS "$OEM_URL" 2>/dev/null)

        if [[ -n "$OEM_DATA" ]] && echo "$OEM_DATA" | jq -e . >/dev/null 2>&1; then
            # Try to get ProductName from Dell OEM data
            OEM_MODEL=$(echo "$OEM_DATA" | jq -r '.Oem.Dell.DellNIC.ProductName // empty')
            if [[ -n "$OEM_MODEL" && "$OEM_MODEL" != "null" ]]; then
                # Clean up the model name (remove MAC address suffix if present)
                MODEL=$(echo "$OEM_MODEL" | sed 's/ - [A-F0-9:]*$//')
            fi
        fi
    fi

    # Use interface speed if available, otherwise adapter speed
    # If link is down and speed is 0, show capable speed in parentheses
    if [[ "$IFACE_SPEED" != "null" && "$IFACE_SPEED" != "N/A" && -n "$IFACE_SPEED" && "$IFACE_SPEED" -gt 0 ]]; then
        # Link is up, show negotiated speed
        if [[ "$IFACE_SPEED" -ge 100000 ]]; then
            DISPLAY_SPEED="$((IFACE_SPEED/1000))GbE"
        elif [[ "$IFACE_SPEED" -ge 1000 ]]; then
            DISPLAY_SPEED="$((IFACE_SPEED/1000))GbE"
        else
            DISPLAY_SPEED="${IFACE_SPEED}Mbps"
        fi
    else
        # Link is down, try to get capable speed from model name or adapter
        CAPABLE_SPEED=""
        # Extract speed from model name (e.g., "100GbE", "25GbE", "Gigabit")
        if [[ "$MODEL" =~ ([0-9]+)GbE ]]; then
            CAPABLE_SPEED="${BASH_REMATCH[1]}GbE"
        elif [[ "$MODEL" =~ Gigabit ]]; then
            CAPABLE_SPEED="1GbE"
        elif [[ "$ADAPTER_SPEED_VAL" != "N/A" && "$ADAPTER_SPEED_VAL" != "null" ]]; then
            CAPABLE_SPEED="$ADAPTER_SPEED_VAL"
        fi

        if [[ -n "$CAPABLE_SPEED" ]]; then
            DISPLAY_SPEED="($CAPABLE_SPEED)"  # Parentheses indicate capable, not negotiated
        else
            DISPLAY_SPEED="N/A"
        fi
    fi

    # Shorten link status
    case "$LINK_STATUS" in
        "LinkUp") LINK="Up" ;;
        "LinkDown") LINK="Down" ;;
        *) LINK="$LINK_STATUS" ;;
    esac

    # Shorten model name for display
    SHORT_MODEL=$(echo "$MODEL" | sed 's/Technologies, Inc.//; s/Corporation//' | cut -c1-30)

    # Guess Linux name
    LINUX_NAME=$(guess_linux_name "$IDRAC_ID")

    printf "%-25s %-20s %-12s %-30s %-7s %-12s %-12s %-10s %-20s\n" \
        "$IDRAC_ID" "$MAC" "$VENDOR" "$SHORT_MODEL" "$PORTS" "$DISPLAY_SPEED" "$SRIOV" "$LINK" "$LINUX_NAME"

done <<< "$INTERFACE_URIS"

echo ""
echo "============================================================"
echo "NOTES:"
echo "  - Vendor identified from MAC address OUI (first 3 bytes)"
echo "  - OS interface names are guesses based on Dell naming conventions"
echo "  - Speed in parentheses (e.g., '(100GbE)') = capable speed (link down)"
echo "  - Speed without parentheses = negotiated speed (link up)"
echo "  - SR-IOV: Yes/No indicates capability; number in parentheses = max VFs"
echo "  - Use MAC address for definitive OS correlation"
echo "============================================================"

# Try to get routing info via racadm netstat (requires sshpass)
echo ""
echo "============================================================"
echo "Host Routing Table (via racadm netstat)"
echo "============================================================"
echo ""

# Check if sshpass is available
if command -v sshpass &> /dev/null; then
    SSH_CMD="sshpass -p $PASSWORD ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR ${USERNAME}@${IDRAC_IP}"

    # Get routing table
    NETSTAT_OUTPUT=$($SSH_CMD "racadm netstat" 2>/dev/null)

    if [[ -n "$NETSTAT_OUTPUT" && ! "$NETSTAT_OUTPUT" =~ "ERROR" ]]; then
        # Filter to show only routing tables, exclude TCP connections
        echo "$NETSTAT_OUTPUT" | sed '/^Active Internet connections/,$d' | sed '/^$/d'
    else
        echo "Unable to retrieve routing info via racadm."
        echo "This may require iDRAC Service Module (iSM) running on the host OS."
    fi

    # Get interface configuration (ifconfig)
    echo ""
    echo "============================================================"
    echo "Host Interface Configuration (via racadm ifconfig)"
    echo "============================================================"
    echo ""

    IFCONFIG_OUTPUT=$($SSH_CMD "racadm ifconfig" 2>/dev/null)

    if [[ -n "$IFCONFIG_OUTPUT" && ! "$IFCONFIG_OUTPUT" =~ "ERROR" ]]; then
        echo "$IFCONFIG_OUTPUT"
    else
        echo "Unable to retrieve interface configuration."
    fi

    # Get ARP table
    echo ""
    echo "============================================================"
    echo "ARP Table (via racadm arp)"
    echo "============================================================"
    echo ""

    ARP_OUTPUT=$($SSH_CMD "racadm arp" 2>/dev/null)

    if [[ -n "$ARP_OUTPUT" && ! "$ARP_OUTPUT" =~ "ERROR" ]]; then
        echo "$ARP_OUTPUT"
    else
        echo "Unable to retrieve ARP table."
    fi

else
    # Try without sshpass using expect-like approach or just inform user
    echo "NOTE: Install 'sshpass' to enable automatic retrieval of:"
    echo "  - Routing table (racadm netstat)"
    echo "  - Interface configuration (racadm ifconfig)"
    echo "  - ARP table (racadm arp)"
    echo ""
    echo "Manual command:"
    echo "  ssh ${USERNAME}@${IDRAC_IP}  (password: ${PASSWORD})"
fi

echo ""
echo "============================================================"
