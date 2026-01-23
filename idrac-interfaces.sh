#!/bin/bash
#
# idrac-interfaces.sh - List network interfaces from iDRAC with guessed Linux names
#
# Usage: ./idrac-interfaces.sh <idrac_ip> [username] [password]
#

set -e

# Default credentials (Dell iDRAC defaults)
IDRAC_IP="${1:-}"
USERNAME="${2:-root}"
PASSWORD="${3:-calvin}"

if [[ -z "$IDRAC_IP" ]]; then
    echo "Usage: $0 <idrac_ip> [username] [password]"
    echo ""
    echo "Example: $0 10.8.35.49 root calvin"
    exit 1
fi

BASE_URL="https://${IDRAC_IP}/redfish/v1/Systems/System.Embedded.1/EthernetInterfaces"

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
        # Slot NICs use PCI naming - we can only give a hint
        echo "ens${slot}f$((port-1)) (or enp*s${slot}f$((port-1)))"
        return
    fi
    
    # Pattern: NIC.Embedded.<adapter>-<port>-<partition> (some older systems)
    if [[ "$idrac_id" =~ ^NIC\.Embedded\.([0-9]+)-([0-9]+)-([0-9]+)$ ]]; then
        local port="${BASH_REMATCH[2]}"
        echo "em${port}"
        return
    fi
    
    echo "unknown"
}

# Curl options
CURL_OPTS="-L -ks -u ${USERNAME}:${PASSWORD}"

echo "============================================================"
echo "iDRAC Network Interfaces - ${IDRAC_IP}"
echo "============================================================"
echo ""

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
printf "%-30s %-20s %-15s %-25s\n" "iDRAC ID" "MAC Address" "Link Status" "Guessed Linux Name"
printf "%-30s %-20s %-15s %-25s\n" "------------------------------" "--------------------" "---------------" "-------------------------"

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
    SPEED=$(echo "$IFACE_DATA" | jq -r '.SpeedMbps // "N/A"')
    
    # Guess Linux name
    LINUX_NAME=$(guess_linux_name "$IDRAC_ID")
    
    printf "%-30s %-20s %-15s %-25s\n" "$IDRAC_ID" "$MAC" "$LINK_STATUS" "$LINUX_NAME"
    
done <<< "$INTERFACE_URIS"

echo ""
echo "============================================================"
echo "NOTE: Linux interface names are GUESSES based on Dell naming"
echo "      conventions. Actual names depend on OS configuration."
echo "      Use MAC address for definitive correlation."
echo "============================================================"
