import os
import logging
import cv2
from ultralytics import YOLO

logger = logging.getLogger("image_analyzer")

# Preload model globally
try:
    logger.info("Loading YOLOv8 model...")
    model = YOLO("yolov8n.pt")
except Exception as e:
    logger.error(f"Failed to load YOLOv8 model: {e}")
    model = None

RISK_CLASSES = ["knife", "gun", "handgun", "pistol", "rifle", "firearm", "weapon", "person"]

def analyze_image(file_path: str, job_id: str, case_id: str, submit_finding_fn, report_progress_fn) -> list:
    logger.info(f"Analyzing image file: {file_path}")
    
    if not os.path.exists(file_path):
        raise FileNotFoundError(f"File not found: {file_path}")
        
    if model is None:
        raise RuntimeError("YOLOv8 model is not loaded")
        
    report_progress_fn(job_id, 20, "running")
    
    # 1. Load image with OpenCV
    img = cv2.imread(file_path)
    if img is None:
        raise ValueError(f"Failed to load image (unsupported format or corrupt file): {file_path}")
        
    report_progress_fn(job_id, 40, "running")
    
    # 2. Resize to max 640px
    h, w = img.shape[:2]
    max_dim = max(h, w)
    if max_dim > 640:
        scale = 640.0 / max_dim
        new_w = int(w * scale)
        new_h = int(h * scale)
        img = cv2.resize(img, (new_w, new_h))
        logger.info(f"Resized image from {w}x{h} to {new_w}x{new_h}")
        h, w = new_h, new_w
        
    report_progress_fn(job_id, 60, "running")
    
    # 3. Perform YOLOv8 inference
    results = model(img, verbose=False)
    
    report_progress_fn(job_id, 80, "running")
    
    findings = []
    
    # 4. Filter risk classes
    if results and len(results) > 0:
        boxes = results[0].boxes
        for box in boxes:
            cls_id = int(box.cls[0])
            label = model.names.get(cls_id, "").lower()
            conf = float(box.conf[0])
            
            is_risk = False
            for rc in RISK_CLASSES:
                if rc in label:
                    is_risk = True
                    break
                    
            if is_risk and conf > 0.4:
                xyxy = box.xyxy[0].tolist()
                x1, y1, x2, y2 = xyxy
                bx = int(x1)
                by = int(y1)
                bw = int(x2 - x1)
                bh = int(y2 - y1)
                
                category = "weapon"
                risk_level = "high"
                
                if "person" in label:
                    category = "violence"
                    risk_level = "low"
                elif conf > 0.8:
                    risk_level = "critical"
                    
                finding = {
                    "case_id": case_id,
                    "job_id": job_id,
                    "worker_type": "image",
                    "category": category,
                    "confidence": conf,
                    "risk_level": risk_level,
                    "evidence": {
                        "bounding_box": {
                            "x": bx,
                            "y": by,
                            "w": bw,
                            "h": bh
                        },
                        "label": label
                    }
                }
                findings.append(finding)
                submit_finding_fn(finding)
                
    report_progress_fn(job_id, 100, "completed")
    return findings
