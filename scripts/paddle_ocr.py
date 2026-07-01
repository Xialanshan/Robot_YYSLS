#!/usr/bin/env python3
import json
import sys


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


def main():
    if len(sys.argv) != 2:
        print("usage: paddle_ocr.py <image_path>", file=sys.stderr)
        return 2

    image_path = sys.argv[1]
    try:
        from paddleocr import PaddleOCR
    except Exception as exc:
        print(f"import paddleocr failed: {exc}", file=sys.stderr)
        return 1

    try:
        ocr = PaddleOCR(use_angle_cls=False, lang="ch")
        raw_result = ocr.ocr(image_path, cls=False)
        print(json.dumps(build_result(raw_result), ensure_ascii=False))
        return 0
    except Exception as exc:
        print(f"run paddleocr failed: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
