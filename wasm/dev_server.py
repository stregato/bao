#!/usr/bin/env python3
import argparse
import base64
import http.server
import socket
import socketserver
import sys
import time
import urllib.parse
import urllib.request
import urllib.error
from pathlib import Path


HOP_BY_HOP = {
    "connection",
    "keep-alive",
    "proxy-authenticate",
    "proxy-authorization",
    "te",
    "trailers",
    "transfer-encoding",
    "upgrade",
}

PROXY_TIMEOUT_SEC = 90
PROXY_MAX_ATTEMPTS = 3


class ProxyHandler(http.server.SimpleHTTPRequestHandler):
    def _is_proxy(self) -> bool:
        return self.path.startswith("/__proxy")

    def _cors(self):
        self.send_header("Access-Control-Allow-Origin", "http://localhost:8000")
        self.send_header("Access-Control-Allow-Methods", "GET,HEAD,PUT,DELETE,OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "*")
        self.send_header("Access-Control-Max-Age", "600")

    def _proxy(self):
        parsed = urllib.parse.urlparse(self.path)
        qs = urllib.parse.parse_qs(parsed.query)
        target = ""
        urlb64 = (qs.get("urlb64") or [""])[0]
        if urlb64:
            try:
                pad = "=" * (-len(urlb64) % 4)
                target = base64.urlsafe_b64decode(urlb64 + pad).decode("utf-8")
            except Exception:
                target = ""
        if not target:
            target = (qs.get("url") or [""])[0]
        if not target:
            self.send_response(400)
            self._cors()
            self.end_headers()
            self.wfile.write(b"missing url/urlb64 parameter")
            return

        target_u = urllib.parse.urlparse(target)
        if target_u.scheme not in ("http", "https"):
            self.send_response(400)
            self._cors()
            self.end_headers()
            self.wfile.write(b"invalid target scheme")
            return

        allowed = target_u.netloc.endswith(".r2.cloudflarestorage.com")
        if not allowed:
            self.send_response(403)
            self._cors()
            self.end_headers()
            self.wfile.write(b"target not allowed")
            return

        body = b""
        if self.command in ("PUT", "POST", "PATCH"):
            n = int(self.headers.get("Content-Length", "0") or "0")
            if n > 0:
                body = self.rfile.read(n)

        out_headers = {}
        for key, value in self.headers.items():
            lk = key.lower()
            if lk in HOP_BY_HOP:
                continue
            if lk in ("host", "origin", "referer", "content-length"):
                continue
            if lk.startswith("x-amz-") or lk in (
                "authorization",
                "content-type",
                "range",
                "if-match",
                "if-none-match",
                "if-modified-since",
                "if-unmodified-since",
            ):
                out_headers[key] = value

        req = urllib.request.Request(target, data=body if body else None, method=self.command)
        for k, v in out_headers.items():
            req.add_header(k, v)
        print(f"[proxy] {self.command} {target}", file=sys.stderr)

        for attempt in range(1, PROXY_MAX_ATTEMPTS + 1):
            try:
                with urllib.request.urlopen(req, timeout=PROXY_TIMEOUT_SEC) as resp:
                    data = resp.read()
                    print(f"[proxy] <- {resp.status} {self.command} {target}", file=sys.stderr)
                    self.send_response(resp.status)
                    self._cors()
                    for k, v in resp.headers.items():
                        if k.lower() in HOP_BY_HOP:
                            continue
                        self.send_header(k, v)
                    self.end_headers()
                    if self.command != "HEAD":
                        self.wfile.write(data)
                    return
            except urllib.error.HTTPError as e:
                data = e.read() if hasattr(e, "read") else b""
                snippet = data[:300].decode("utf-8", errors="replace")
                print(f"[proxy] <- {e.code} {self.command} {target}\n{snippet}", file=sys.stderr)
                self.send_response(e.code)
                self._cors()
                for k, v in e.headers.items():
                    if k.lower() in HOP_BY_HOP:
                        continue
                    self.send_header(k, v)
                self.end_headers()
                if self.command != "HEAD":
                    self.wfile.write(data)
                return
            except Exception as e:
                msg = str(e)
                timed_out = isinstance(e, (TimeoutError, socket.timeout)) or "timed out" in msg.lower()
                if timed_out and attempt < PROXY_MAX_ATTEMPTS:
                    backoff = 0.4 * attempt
                    print(f"[proxy] timeout attempt {attempt}/{PROXY_MAX_ATTEMPTS} for {self.command} {target}; retry in {backoff:.1f}s", file=sys.stderr)
                    time.sleep(backoff)
                    continue
                print(f"[proxy] xx {self.command} {target}: {e}", file=sys.stderr)
                self.send_response(502)
                self._cors()
                self.end_headers()
                self.wfile.write(str(e).encode("utf-8"))
                return

    def do_OPTIONS(self):
        if self._is_proxy():
            self.send_response(204)
            self._cors()
            self.end_headers()
            return
        self.send_response(204)
        self.end_headers()

    def do_GET(self):
        if self._is_proxy():
            self._proxy()
            return
        super().do_GET()

    def do_HEAD(self):
        if self._is_proxy():
            self._proxy()
            return
        super().do_HEAD()

    def do_PUT(self):
        if self._is_proxy():
            self._proxy()
            return
        self.send_error(405, "Method Not Allowed")

    def do_DELETE(self):
        if self._is_proxy():
            self._proxy()
            return
        self.send_error(405, "Method Not Allowed")


def main():
    parser = argparse.ArgumentParser(description="Bao WASM dev server with transparent proxy")
    parser.add_argument("--port", type=int, default=8000)
    parser.add_argument("--root", type=str, default=".")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    handler = lambda *a, **kw: ProxyHandler(*a, directory=str(root), **kw)
    with socketserver.TCPServer(("", args.port), handler) as httpd:
        print(f"Serving {root} at http://localhost:{args.port}/wasm/index.html")
        print(f"Proxy endpoint: http://localhost:{args.port}/__proxy?url=<encoded-url>")
        httpd.serve_forever()


if __name__ == "__main__":
    main()
