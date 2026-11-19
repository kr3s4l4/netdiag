#!/usr/bin/env python3
"""Agente SNMP `pass` para el laboratorio F46.

Sirve system, interfaces, IP-MIB, Q-BRIDGE y BRIDGE-MIB desde un único
registro prioritario, sin depender de los módulos built-in de snmpd.

  -g OID -> "OID TIPO VALOR"
  -n OID -> siguiente OID servido (walk)
"""
import os
import sys

REG = ".1.3.6.1.2.1"
STATE = "/state/moved"

GW_MAC_A = "00:11:22:33:44:55"
GW_MAC_B = "66:77:88:99:aa:bb"
ROGUE_MAC = "de:ad:be:ef:00:53"
SELF_MAC = "aa:bb:cc:dd:ee:0a"


def mac_octets(mac):
    return ".".join(str(int(x, 16)) for x in mac.split(":"))


def moved():
    return os.path.exists(STATE)


def gateway_mac():
    return GW_MAC_B if moved() else GW_MAC_A


def qfdb_port():
    return 7 if moved() else 3


def rows():
    d = {}

    def put(oid, typ, val):
        d[oid] = (typ, str(val))

    # system (el colector exige descr/sysName no vacíos)
    put(f"{REG}.1.1.0", "s", "NetDiag Lab Switch (snmpd)")
    put(f"{REG}.1.2.0", "o", "1.3.6.1.4.1.8072.3.2.10")
    put(f"{REG}.1.3.0", "u", 123456)
    put(f"{REG}.1.4.0", "s", "lab@example.test")
    put(f"{REG}.1.5.0", "s", "lab-switch")
    put(f"{REG}.1.6.0", "s", "Laboratorio NetDiag")

    # interfaces: índices 3 y 7 para correlacionar FDB con LLDP
    for idx, name, mac in ((3, "Gi0/3", ROGUE_MAC), (7, "Gi0/7", SELF_MAC)):
        put(f"{REG}.2.2.1.2.{idx}", "s", name)
        put(f"{REG}.31.1.1.1.1.{idx}", "s", name)
        put(f"{REG}.2.2.1.6.{idx}", "x", mac)
        put(f"{REG}.2.2.1.8.{idx}", "i", 1)
        put(f"{REG}.2.2.1.5.{idx}", "u", 1000000000)

    # ipAddrTable: el índice real es la propia IP.
    put(f"{REG}.4.20.1.1.172.20.50.161", "a", "172.20.50.161")
    put(f"{REG}.4.20.1.2.172.20.50.161", "i", 3)
    put(f"{REG}.4.20.1.3.172.20.50.161", "a", "255.255.255.0")

    # ipNetToMediaTable: la MAC del gateway cambia entre polls.
    for ip, mac in (
        ("172.20.50.1", gateway_mac()),
        ("172.20.50.10", SELF_MAC),
        ("172.20.50.53", ROGUE_MAC),
    ):
        suffix = f"3.{ip}"
        put(f"{REG}.4.22.1.1.{suffix}", "i", 3)
        put(f"{REG}.4.22.1.2.{suffix}", "x", mac)
        put(f"{REG}.4.22.1.3.{suffix}", "a", ip)
        put(f"{REG}.4.22.1.4.{suffix}", "i", 3)

    # ipNetToPhysicalTable (RFC 4293)
    for ip, mac in (("172.20.50.1", gateway_mac()), ("172.20.50.10", SELF_MAC)):
        suffix = f"3.1.4.{ip}"
        put(f"{REG}.4.35.1.1.{suffix}", "i", 3)
        put(f"{REG}.4.35.1.3.{suffix}", "a", ip)
        put(f"{REG}.4.35.1.4.{suffix}", "x", mac)
        put(f"{REG}.4.35.1.6.{suffix}", "i", 3)

    # Q-BRIDGE: VLAN + FDB en dos VLAN para forzar conflicts.
    put(f"{REG}.17.7.1.4.3.1.1.10", "s", "Lab-VLAN-10")
    put(f"{REG}.17.7.1.4.3.1.1.20", "s", "Lab-VLAN-20")
    put(f"{REG}.17.1.4.1.2.3", "s", "Gi0/3")
    put(f"{REG}.17.1.4.1.2.7", "s", "Gi0/7")
    for vlan in (10, 20):
        suffix = f"{vlan}.{mac_octets(ROGUE_MAC)}"
        put(f"{REG}.17.7.1.2.2.1.1.{suffix}", "x", ROGUE_MAC)
        put(f"{REG}.17.7.1.2.2.1.2.{suffix}", "i", qfdb_port())
        put(f"{REG}.17.7.1.2.2.1.3.{suffix}", "i", 3)

    # BRIDGE-MIB
    suffix = mac_octets(SELF_MAC)
    put(f"{REG}.17.4.3.1.1.{suffix}", "x", SELF_MAC)
    put(f"{REG}.17.4.3.1.2.{suffix}", "i", 7)
    put(f"{REG}.17.4.3.1.3.{suffix}", "i", 3)

    return d


def oid_key(oid):
    return tuple(int(p) for p in oid.strip(".").split("."))


def main():
    if len(sys.argv) != 3 or sys.argv[1] not in ("-g", "-n"):
        return
    mode, requested = sys.argv[1], sys.argv[2]
    data = rows()
    if mode == "-g":
        hit = data.get(requested)
        if hit:
            print(f"{requested} {hit[0]} {hit[1]}")
        return
    needle = oid_key(requested)
    for oid in sorted(data, key=oid_key):
        if oid.startswith(REG + ".") and oid_key(oid) > needle:
            typ, val = data[oid]
            print(f"{oid} {typ} {val}")
            return


if __name__ == "__main__":
    main()
