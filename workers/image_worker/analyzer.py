import os
import logging
import cv2
import requests
from ultralytics import YOLO

logger = logging.getLogger("image_analyzer")

# Preload models globally
try:
    logger.info("Loading YOLOv8 COCO model...")
    coco_model = YOLO("yolov8n.pt")
except Exception as e:
    logger.error(f"Failed to load YOLOv8 COCO model: {e}")
    coco_model = None

THREAT_MODEL_PATH = os.path.join(os.path.dirname(__file__), "threats_yolov8n.pt")

try:
    logger.info("Loading YOLOv8 Threat model...")
    if not os.path.exists(THREAT_MODEL_PATH):
        logger.info(f"Threat model not found at {THREAT_MODEL_PATH}. Downloading from Hugging Face...")
        r = requests.get("https://huggingface.co/Subh775/Threat-Detection-YOLOv8n/resolve/main/weights/best.pt", timeout=30)
        r.raise_for_status()
        with open(THREAT_MODEL_PATH, "wb") as f:
            f.write(r.content)
        logger.info("Threat model downloaded successfully.")
    
    threat_model = YOLO(THREAT_MODEL_PATH)
except Exception as e:
    logger.error(f"Failed to load YOLOv8 Threat model: {e}")
    threat_model = None


def analyze_image(file_path: str, job_id: str, case_id: str, submit_finding_fn, report_progress_fn) -> list:
    logger.info(f"Analyzing image file: {file_path}")
    
    if not os.path.exists(file_path):
        raise FileNotFoundError(f"File not found: {file_path}")
        
    if coco_model is None and threat_model is None:
        raise RuntimeError("Neither COCO nor Threat model is loaded")
        
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
    
    findings = []
    
    # 3. Perform COCO model inference for "person"
    if coco_model is not None:
        logger.info("Running COCO model inference...")
        coco_results = coco_model(img, verbose=False)
        if coco_results and len(coco_results) > 0:
            boxes = coco_results[0].boxes
            for box in boxes:
                cls_id = int(box.cls[0])
                label = coco_model.names.get(cls_id, "").lower()
                conf = float(box.conf[0])
                
                # Only care about person detection in COCO to identify violence context
                if label == "person" and conf > 0.4:
                    xyxy = box.xyxy[0].tolist()
                    x1, y1, x2, y2 = xyxy
                    bx = int(x1)
                    by = int(y1)
                    bw = int(x2 - x1)
                    bh = int(y2 - y1)
                    
                    finding = {
                        "case_id": case_id,
                        "job_id": job_id,
                        "worker_type": "image",
                        "category": "violence",
                        "confidence": conf,
                        "risk_level": "low",
                        "evidence": {
                            "bounding_box": {
                                "x": bx,
                                "y": by,
                                "w": bw,
                                "h": bh
                            },
                            "label": "person"
                        }
                    }
                    findings.append(finding)
                    submit_finding_fn(finding)
                    
    report_progress_fn(job_id, 80, "running")
    
    # 4. Perform Threat model inference for weapon detections
    if threat_model is not None:
        logger.info("Running Threat model inference...")
        threat_results = threat_model(img, verbose=False)
        if threat_results and len(threat_results) > 0:
            boxes = threat_results[0].boxes
            for box in boxes:
                cls_id = int(box.cls[0])
                label = threat_model.names.get(cls_id, "").lower()
                conf = float(box.conf[0])
                
                # Threat model classes: 0: Gun, 1: explosion, 2: grenade, 3: knife
                # Lower threshold to 0.25 for threat detections
                if conf > 0.25:
                    xyxy = box.xyxy[0].tolist()
                    x1, y1, x2, y2 = xyxy
                    bx = int(x1)
                    by = int(y1)
                    bw = int(x2 - x1)
                    bh = int(y2 - y1)
                    
                    category = "weapon"
                    risk_level = "high"
                    
                    if "explosion" in label:
                        category = "violence"
                        risk_level = "critical"
                    elif "grenade" in label:
                        category = "weapon"
                        risk_level = "critical"
                    elif "gun" in label:
                        category = "weapon"
                        risk_level = "critical" if conf > 0.60 else "high"
                    elif "knife" in label:
                        category = "weapon"
                        risk_level = "high" if conf > 0.60 else "medium"
                    else:
                        if conf > 0.60:
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

