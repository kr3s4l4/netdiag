#!/usr/bin/env python3
"""Agente SNMP v2c mínimo para el laboratorio F46.

Maneja GetRequest y GetBulkRequest con BER propio. Las filas viven en
snmp_pass.rows(), para que el agente y el script `pass` compartan datos.
"""
import socket
import struct

import snmp_pass

TABLE = []
for _oid, (_typ, _val) in snmp_pass.rows().items():
    TABLE.append((tuple(int(p) for p in _oid.strip(".").split(".")), _typ, str(_val)))
TABLE.sort(key=lambda r: r[0])


def enc_len(n):
    if n < 128:
        return bytes([n])
    b = n.to_bytes((n.bit_length() + 7) // 8, "big")
    return bytes([0x80 | len(b)]) + b


def tlv(tag, content):
    return bytes([tag]) + enc_len(len(content)) + content


def base128(n):
    out = []
    while True:
        out.append(n & 0x7F)
        n >>= 7
        if n == 0:
            break
    out.reverse()
    return bytes([x | 0x80 for x in out[:-1]] + [out[-1]])


def enc_oid(parts):
    first = parts[0] * 40 + parts[1]
    body = base128(first) + b"".join(base128(p) for p in parts[2:])
    return tlv(0x06, body)


def enc_int(n):
    if n == 0:
        b = b"\x00"
    else:
        size = (n.bit_length() + 7) // 8
        if n & (1 << (n.bit_length() - 1)):
            size += 1
        b = n.to_bytes(size, "big", signed=True)
    return tlv(0x02, b)


def enc_value(typ, val):
    if typ == "s":
        return tlv(0x04, val.encode())
    if typ == "i":
        return enc_int(int(val))
    if typ == "u":
        n = int(val)
        return tlv(0x41, n.to_bytes(max(1, (n.bit_length() + 7) // 8), "big"))
    if typ == "a":
        return tlv(0x40, socket.inet_aton(val))
    if typ == "x":
        return tlv(0x04, bytes.fromhex(val.replace(":", "")))
    if typ == "o":
        return enc_oid(tuple(int(p) for p in val.strip(".").split(".")))
    return tlv(0x05, b"")


def dec_len(data, pos):
    b = data[pos]
    pos += 1
    if b < 128:
        return b, pos
    n = b & 0x7F
    return int.from_bytes(data[pos : pos + n], "big"), pos + n


def dec_int(data, pos):
    pos += 1
    length, pos = dec_len(data, pos)
    return int.from_bytes(data[pos : pos + length], "big", signed=True), pos + length


def dec_oid(data, pos):
    pos += 1
    length, pos = dec_len(data, pos)
    content = data[pos : pos + length]
    pos += length
    numbers = []
    acc = 0
    for byte in content:
        acc = (acc << 7) | (byte & 0x7F)
        if not byte & 0x80:
            numbers.append(acc)
            acc = 0
    if not numbers:
        return (), pos
    a, b = numbers[0] // 40, numbers[0] % 40
    if a > 2:
        a, b = 2, numbers[0] - 80
    return (a, b) + tuple(numbers[1:]), pos


def skip_tlv(data, pos):
    pos += 1
    length, pos = dec_len(data, pos)
    return pos + length


def get_next(oid):
    for candidate, typ, val in TABLE:
        if candidate > oid:
            return candidate, typ, val
    return oid, "end", ""


def handle(data, addr, sock):
    if not data or data[0] != 0x30:
        return
    _, pos = dec_len(data, 1)
    version, pos = dec_int(data, pos)
    if data[pos] != 0x04:
        return
    clen, pos = dec_len(data, pos + 1)
    community = data[pos : pos + clen].decode(errors="replace")
    pos += clen
    pdu_tag = data[pos]
    if pdu_tag not in (0xA0, 0xA1, 0xA5):
        return
    _, pos = dec_len(data, pos + 1)
    request_id, pos = dec_int(data, pos)
    non_repeaters, max_repetitions = 0, 0
    if pdu_tag == 0xA5:
        non_repeaters, pos = dec_int(data, pos)
        max_repetitions, pos = dec_int(data, pos)
    else:
        _, pos = dec_int(data, pos)  # error status
        _, pos = dec_int(data, pos)  # error index
    if data[pos] != 0x30:
        return
    vlen, pos = dec_len(data, pos + 1)
    vend = pos + vlen
    requested = []
    while pos < vend:
        if data[pos] != 0x30:
            break
        _, pos = dec_len(data, pos + 1)
        oid, pos = dec_oid(data, pos)
        pos = skip_tlv(data, pos)
        requested.append(oid)

    out = []
    for i, oid in enumerate(requested):
        if pdu_tag in (0xA1, 0xA5) and i >= non_repeaters:
            current = oid
            for _ in range(max(1, max_repetitions)):
                nxt, typ, val = get_next(current)
                if typ == "end":
                    out.append((nxt, "end", ""))
                    break
                out.append((nxt, typ, val))
                current = nxt
        else:
            hit = next((r for r in TABLE if r[0] == oid), None)
            if hit:
                out.append(hit)
            else:
                out.append((oid, "end", ""))

    varbinds = b""
    for oid, typ, val in out:
        value = tlv(0x80, b"") if typ == "end" else enc_value(typ, val)
        varbinds += tlv(0x30, enc_oid(oid) + value)
    body = enc_int(request_id) + enc_int(0) + enc_int(0) + tlv(0x30, varbinds)
    pdu = tlv(0xA2, body)
    message = tlv(0x30, enc_int(1) + tlv(0x04, community.encode()) + pdu)
    sock.sendto(message, addr)


def main():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(("0.0.0.0", 161))
    print(f"agente SNMP listo: {len(TABLE)} OIDs", flush=True)
    while True:
        data, addr = sock.recvfrom(65535)
        try:
            handle(data, addr, sock)
        except Exception as exc:  # no derribar el agente por un paquete malo
            print(f"error: {exc}", flush=True)


if __name__ == "__main__":
    main()
