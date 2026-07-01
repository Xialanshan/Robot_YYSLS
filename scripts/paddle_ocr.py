#!/usr/bin/env python3
import argparse
import base64
import json
import sys
import tempfile
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


OCR_ENGINE = None


def load_engine():
    global OCR_ENGINE
    if OCR_ENGINE is not None:
        return OCR_ENGINE
    try:
        from paddleocr import PaddleOCR
    except Exception as exc:
        raise RuntimeError(f"import paddleocr failed: {exc}") from exc
    OCR_ENGINE = PaddleOCR(use_angle_cls=False, lang="ch")
    return OCR_ENGINE


def build_result(raw_result):
    items = []
    for page in raw_result or []:
        if page is None:
            continue
        for line in page:
            if not line or len(line) < 2:
                continue
            polygon = []
            for point in line[0]:
                if len(point) >= 2:
                    polygon.append({"x": int(point[0]), "y": int(point[1])})
            text = ""
            confidence = 0.0
            detail = line[1]
            if isinstance(detail, (list, tuple)) and len(detail) >= 2:
                text = str(detail[0])
                confidence = float(detail[1] or 0.0)
            items.append({
                "text": text,
                "confidence": confidence,
                "polygon": polygon,
            })
    return {"provider": "paddle", "items": items}


def recognize_image_bytes(image_bytes):
    with tempfile.NamedTemporaryFile(delete=False, suffix=".png") as temp_file:
        temp_file.write(image_bytes)
        image_path = temp_file.name
    try:
        raw_result = load_engine().ocr(image_path, cls=False)
        return build_result(raw_result)
    finally:
        try:
            Path(image_path).unlink(missing_ok=True)
        except Exception:
            pass


class OCRHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/healthz":
            self.send_response(200)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.end_headers()
            self.wfile.write(b'{"status":"ok"}')
            return
        self.send_response(404)
        self.end_headers()

    def do_POST(self):
        if self.path != "/ocr":
            self.send_response(404)
            self.end_headers()
            return
        length = int(self.headers.get("Content-Length", "0"))
        body = self.rfile.read(length)
        try:
            payload = json.loads(body.decode("utf-8"))
            image_base64 = payload.get("image_base64", "")
            if not image_base64:
                raise ValueError("image_base64 is required")
            image_bytes = base64.b64decode(image_base64)
            result = recognize_image_bytes(image_bytes)
            response = json.dumps(result, ensure_ascii=False).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(response)))
            self.end_headers()
            self.wfile.write(response)
        except Exception as exc:
            message = json.dumps({"error": str(exc)}, ensure_ascii=False).encode("utf-8")
            self.send_response(500)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Content-Length", str(len(message)))
            self.end_headers()
            self.wfile.write(message)

    def log_message(self, format, *args):
        return


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=18081)
    args = parser.parse_args()

    try:
        load_engine()
    except Exception as exc:
        print(str(exc), file=sys.stderr)
        return 1

    server = ThreadingHTTPServer((args.host, args.port), OCRHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        return 0
    finally:
        server.server_close()


if __name__ == "__main__":
    raise SystemExit(main())
