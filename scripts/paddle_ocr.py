#!/usr/bin/env python3
import argparse
import base64
import json
import os
import sys
import tempfile
import traceback
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

# Reduce CPU-runtime incompatibility and memory pressure on small servers.
os.environ.setdefault("FLAGS_enable_pir_api", "0")
os.environ.setdefault("FLAGS_use_mkldnn", "0")
os.environ.setdefault("OMP_NUM_THREADS", "1")
os.environ.setdefault("MKL_NUM_THREADS", "1")


OCR_ENGINE = None


def log_debug(message):
    print(f"[paddle_ocr] {message}", file=sys.stderr, flush=True)


def load_engine():
    global OCR_ENGINE
    if OCR_ENGINE is not None:
        return OCR_ENGINE
    try:
        from paddleocr import PaddleOCR
    except Exception as exc:
        raise RuntimeError(f"import paddleocr failed: {exc}") from exc
    candidates = [
        {
            "use_doc_orientation_classify": False,
            "use_doc_unwarping": False,
            "use_textline_orientation": False,
            "lang": "ch",
        },
        {
            "use_doc_orientation_classify": False,
            "use_doc_unwarping": False,
            "use_angle_cls": False,
            "lang": "ch",
        },
        {
            "use_textline_orientation": False,
            "lang": "ch",
        },
        {
            "use_angle_cls": False,
            "lang": "ch",
        },
        {
            "lang": "ch",
        },
    ]
    last_error = None
    for kwargs in candidates:
        try:
            OCR_ENGINE = PaddleOCR(**kwargs)
            return OCR_ENGINE
        except TypeError as exc:
            last_error = exc
            continue
        except Exception as exc:
            # Keep trying smaller configurations before giving up.
            last_error = exc
            continue
    if last_error is not None:
        raise RuntimeError(f"initialize paddleocr failed: {last_error}") from last_error
    return OCR_ENGINE


def build_legacy_result(raw_result):
    items = []
    for page in raw_result or []:
        if page is None:
            continue
        if isinstance(page, dict):
            texts = page.get("rec_texts") or page.get("text") or []
            scores = page.get("rec_scores") or page.get("scores") or []
            polys = page.get("rec_polys")
            if polys is None:
                polys = page.get("dt_polys")
            if polys is None:
                polys = []
            for idx, text in enumerate(texts):
                raw_polygon = polys[idx] if idx < len(polys) else []
                polygon = []
                for point in raw_polygon if raw_polygon is not None else []:
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
        texts = _value_from_result(page, "rec_texts", [])
        if texts is None:
            texts = _value_from_result(page, "text", [])
        if texts is None:
            texts = []
        scores = _value_from_result(page, "rec_scores", [])
        if scores is None:
            scores = _value_from_result(page, "scores", [])
        if scores is None:
            scores = []
        polys = _value_from_result(page, "rec_polys", None)
        if polys is None:
            polys = _value_from_result(page, "dt_polys", [])
        if polys is None:
            polys = []
        for idx, text in enumerate(texts):
            raw_polygon = polys[idx] if idx < len(polys) else []
            polygon = []
            for point in raw_polygon if raw_polygon is not None else []:
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


def has_meaningful_text(result):
    items = result.get("items", []) if isinstance(result, dict) else []
    for item in items:
        text = str(item.get("text", "")).strip()
        if text and text != "[]":
            return True
    return False


def run_ocr(engine, image_path):
    last_error = None
    for method_name in ("predict", "ocr"):
        method = getattr(engine, method_name, None)
        if method is None:
            continue
        try:
            result = method(image_path)
            log_debug(f"{method_name} raw type={type(result)!r}")
            log_debug(f"{method_name} raw repr={repr(result)[:2000]}")
            if method_name == "predict":
                parsed = build_predict_result(result)
            else:
                parsed = build_legacy_result(result)
            log_debug(f"{method_name} parsed items={len(parsed.get('items', []))}")
            if has_meaningful_text(parsed):
                return parsed
            last_error = RuntimeError(f"{method_name} returned empty text results")
            continue
        except TypeError as exc:
            log_debug(f"{method_name} type error={exc!r}")
            last_error = exc
            continue
        except Exception as exc:
            log_debug(f"{method_name} exception={exc!r}")
            # Some PaddleOCR builds raise generic std::exception for specific
            # images or operator combinations. Try the next method and keep
            # the original exception chain available to the caller.
            last_error = exc
            continue
    if last_error is not None:
        raise RuntimeError(f"paddle OCR inference failed: {last_error}") from last_error
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
            try:
                self.send_response(500)
                self.send_header("Content-Type", "application/json; charset=utf-8")
                self.send_header("Content-Length", str(len(message)))
                self.end_headers()
                self.wfile.write(message)
            except BrokenPipeError:
                return

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
        traceback.print_exc()
        return 1

    log_debug("engine initialized")

    server = ThreadingHTTPServer((args.host, args.port), OCRHandler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        return 0
    finally:
        server.server_close()


if __name__ == "__main__":
    raise SystemExit(main())
