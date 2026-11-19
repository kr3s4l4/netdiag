#!/usr/bin/env python3
"""Generador de tráfico controlado para validar F47 con captura real."""
import argparse
import random
import string
import time

from scapy.all import ARP, DNS, DNSQR, Ether, ICMP, IP, TCP, UDP, send, sendp

TARGET = "172.20.50.10"
ROGUE_IP = "172.20.50.53"
ROGUE_MAC = "de:ad:be:ef:00:53"
GW_IP = "172.20.50.1"
SCANNER_IP = "172.20.50.77"


def garp():
    # ARP request con source IP == target IP: GARP repetido.
    pkt = Ether(src=ROGUE_MAC, dst="ff:ff:ff:ff:ff:ff") / ARP(
        psrc=ROGUE_IP, pdst=ROGUE_IP, hwsrc=ROGUE_MAC, op="who-has"
    )
    sendp(pkt, iface="eth0", count=6, inter=0.25, verbose=0)


def icmp_redirect():
    inner = IP(src="172.20.50.100", dst=TARGET) / ICMP()
    pkt = IP(src=GW_IP, dst=TARGET) / ICMP(type=5, code=1, gw=ROGUE_IP) / inner
    send(pkt, iface="eth0", count=3, inter=0.3, verbose=0)


def syn_sweep():
    for port in range(1024, 1044):
        pkt = IP(src=SCANNER_IP, dst=TARGET) / TCP(
            sport=40000 + port, dport=port, flags="S"
        )
        send(pkt, iface="eth0", verbose=0)


def dns_tunnel():
    for i in range(15):
        label = "".join(random.choice(string.ascii_lowercase + string.digits) for _ in range(28))
        qname = f"{label}.tunnel.example.test"
        pkt = IP(src=SCANNER_IP, dst=TARGET) / UDP(sport=41000 + i, dport=53) / DNS(
            qd=DNSQR(qname=qname)
        )
        send(pkt, iface="eth0", verbose=0)


def burst():
    garp()
    icmp_redirect()
    syn_sweep()
    dns_tunnel()
    print("burst enviado", flush=True)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--loop", action="store_true")
    ap.add_argument("--rounds", type=int, default=1)
    args = ap.parse_args()

    if args.loop:
        while True:
            burst()
            time.sleep(4)
    for _ in range(args.rounds):
        burst()


if __name__ == "__main__":
    main()
