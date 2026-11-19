#!/usr/bin/env python3
"""OFFER DHCP real para ejercitar el detector pasivo de F47."""
import time

from scapy.all import BOOTP, DHCP, Ether, IP, UDP, sendp

ROGUE_IP = "172.20.50.53"
ROGUE_MAC = "de:ad:be:ef:00:53"
CLIENT_MAC = "aa:bb:cc:dd:ee:0a"


def offer():
    pkt = (
        Ether(src=ROGUE_MAC, dst="ff:ff:ff:ff:ff:ff")
        / IP(src=ROGUE_IP, dst="255.255.255.255")
        / UDP(sport=67, dport=68)
        / BOOTP(
            op=2,
            xid=0x5EED1234,
            yiaddr="172.20.50.100",
            siaddr=ROGUE_IP,
            chaddr=bytes.fromhex(CLIENT_MAC.replace(":", "")) + b"\x00" * 10,
        )
        / DHCP(
            options=[
                ("message-type", "offer"),
                ("server_id", ROGUE_IP),
                ("router", "172.20.50.1"),
                ("lease_time", 3600),
                "end",
            ]
        )
    )
    sendp(pkt, iface="eth0", verbose=0)


while True:
    offer()
    time.sleep(1)
