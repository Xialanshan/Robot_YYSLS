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
    try:
        OCR_ENGINE = PaddleOCR(use_textline_orientation=False, lang="ch")
    except TypeError:
        OCR_ENGINE = PaddleOCR(use_angle_cls=False, lang="ch")
    return OCR_ENGINE


def build_legacy_result(raw_result):
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


def _value_from_result(result, key, default=None):
    if isinstance(result, dict):
        return result.get(key, default)
    if hasattr(result, key):
        return getattr(result, key)
    if hasattr(result, "res") and isinstance(result.res, dict):
        return result.res.get(key, default)
    return default


def build_predict_result(raw_result):
    items = []
    for page in raw_result or []:
        texts = _value_from_result(page, "rec_texts", []) or []
        scores = _value_from_result(page, "rec_scores", []) or []
        polys = _value_from_result(page, "rec_polys", None)
        if polys is None:
            polys = _value_from_result(page, "dt_polys", []) or []
        for idx, text in enumerate(texts):
            raw_polygon = polys[idx] if idx < len(polys) else []
            polygon = []
            for point in raw_polygon or []:
                if isinstance(point, dict):
                    x = point.get("x", 0)
                    y = point.get("y", 0)
                else:
                    if len(point) < 2:
                        continue
                    x, y = point[0], point[1]
                polygon.append({"x": int(x), "y": int(y)})
            confidence = 0.0
            if idx < len(scores):
                try:
                    confidence = float(scores[idx] or 0.0)
                except Exception:
                    confidence = 0.0
            items.append({
                "text": str(text),
                "confidence": confidence,
                "polygon": polygon,
            })
    return {"provider": "paddle", "items": items}


def run_ocr(engine, image_path):
    last_error = None
    for method_name in ("predict", "ocr"):
        method = getattr(engine, method_name, None)
        if method is None:
            continue
        try:
            result = method(image_path)
            if method_name == "predict":
                return build_predict_result(result)
            return build_legacy_result(result)
        except TypeError as exc:
            last_error = exc
            continue
    if last_error is not None:
        raise last_error
    raise RuntimeError("PaddleOCR engine does not provide a usable predict/ocr method")


def recognize_image_bytes(image_bytes):
    with tempfile.NamedTemporaryFile(delete=False, suffix=".png") as temp_file:
        temp_file.write(image_bytes)
        image_path = temp_file.name
    try:
        return run_ocr(load_engine(), image_path)
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
