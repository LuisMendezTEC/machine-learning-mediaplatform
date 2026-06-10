import os
import logging
import re
import whisper
from transformers import pipeline

logger = logging.getLogger("audio_analyzer")

# Preload models globally
try:
    logger.info("Loading Whisper small model...")
    whisper_model = whisper.load_model("small")
except Exception as e:
    logger.error(f"Failed to load Whisper model: {e}")
    whisper_model = None

try:
    logger.info("Loading sentiment pipeline for audio...")
    sentiment_pipeline = pipeline(
        "sentiment-analysis", 
        model="lxyuan/distilbert-base-multilingual-cased-sentiments-student",
        device=-1 # CPU
    )
except Exception as e:
    logger.error(f"Failed to load sentiment pipeline: {e}")
    sentiment_pipeline = None

RISK_KEYWORDS = [
    "bomba", "terrorista", "matar", "secuestro", "droga", "cocaína", "arma",
    "pistola", "cuchillo", "violencia", "ataque", "muerte", "asesinar", "violación",
    "bomb", "terrorist", "kill", "kidnap", "drug", "cocaine", "weapon",
    "gun", "knife", "violence", "attack", "death", "murder", "rape", "threat", "amenaza",
    "atacar", "quemar", "incendiar", "destruir", "romper", "dañar", "lanzar", "explotar", 
    "asaltar", "fuego", "explosivo", "explosivos", "incendio", "aerosoles", "botellas",
    "burn", "destroy", "break", "damage", "explode", "assault", "fire", "explosive", 
    "explosives", "arson", "vandalism", "throw",
    "fraude", "estafa", "scam", "phishing", "contraseña", "password", "clave", "banco", 
    "tarjeta", "banca", "dinero", "pin", "cvv"
]

def format_timestamp(seconds: float) -> str:
    h = int(seconds // 3600)
    m = int((seconds % 3600) // 60)
    s = int(seconds % 60)
    return f"{h:02d}:{m:02d}:{s:02d}"

def analyze_audio(file_path: str, job_id: str, case_id: str, submit_finding_fn, report_progress_fn) -> list:
    logger.info(f"Analyzing audio file: {file_path}")
    
    if not os.path.exists(file_path):
        raise FileNotFoundError(f"File not found: {file_path}")
        
    if whisper_model is None:
        raise RuntimeError("Whisper model is not loaded")
        
    report_progress_fn(job_id, 20, "running")
    
    # 1. Transcribe audio to text
    logger.info("Running Whisper transcription...")
    result = whisper_model.transcribe(
        file_path,
        language="es",
        temperature=0.0,
        initial_prompt="Transcripción de audios de seguridad, llamadas de emergencia, denuncias de extorsión, robo, estafa, vandalismo, pandillas, armas como AK-47 o fusiles, y tráfico de drogas."
    )
    
    report_progress_fn(job_id, 60, "running")
    
    segments = result.get("segments", [])
    total_segments = len(segments)
    logger.info(f"Whisper completed. Found {total_segments} segments.")
    
    findings = []
    
    # 2. Process each segment
    for idx, seg in enumerate(segments):
        if total_segments > 0:
            progress_pct = int(60 + (idx / total_segments) * 35)
            report_progress_fn(job_id, progress_pct, "running")
            
        text = seg.get("text", "").strip()
        if not text:
            continue
            
        start_time = seg.get("start", 0.0)
        timestamp = format_timestamp(start_time)
        text_lower = text.lower()
        
        # A. Check risk keywords
        matched_keywords = []
        for kw in RISK_KEYWORDS:
            if re.search(rf"\b{re.escape(kw)}\b", text_lower):
                matched_keywords.append(kw)
        if matched_keywords:
            risk_level = "medium"
            category = "keyword"
            
            weapons = ["cuchillo", "pistola", "arma", "knife", "gun", "weapon", "explosivo", "explosivos", "explosive", "explosives"]
            violence = [
                "violencia", "violence", "violación", "rape", "atacar", "attack", "quemar", "burn", "incendiar", "arson", 
                "destruir", "destroy", "romper", "break", "dañar", "damage", "lanzar", "throw", "explotar", "explode", 
                "asaltar", "assault", "fuego", "fire", "incendio", "vandalismo", "vandalism", "aerosoles", "botellas", "piedras"
            ]
            threats = ["matar", "asesinar", "secuestro", "kill", "murder", "kidnap", "threat", "amenaza"]
            scam = ["fraude", "estafa", "scam", "phishing", "contraseña", "password", "clave", "banco", "tarjeta", "banca", "dinero", "pin", "cvv"]

            if any(w in matched_keywords for w in weapons):
                category = "weapon"
                risk_level = "high"
            elif any(t in matched_keywords for t in threats):
                category = "threat"
                risk_level = "high"
            elif any(s in matched_keywords for s in scam):
                category = "scam"
                risk_level = "high"
            elif any(v in matched_keywords for v in violence):
                category = "violence"
                risk_level = "high"
                
            finding = {
                "case_id": case_id,
                "job_id": job_id,
                "worker_type": "audio",
                "category": category,
                "confidence": 1.0,
                "risk_level": risk_level,
                "evidence": {
                    "transcription": text,
                    "timestamp": timestamp,
                    "keywords": matched_keywords
                }
            }
            findings.append(finding)
            submit_finding_fn(finding)
            
        # B. Check sentiment
        if sentiment_pipeline and len(text) > 3:
            try:
                truncated_text = text[:400]
                res = sentiment_pipeline(truncated_text)[0]
                label = res["label"].lower()
                score = res["score"]
                
                if label == "negative" and score > 0.65:
                    risk_level = "low"
                    if score > 0.85:
                        risk_level = "medium"
                    if score > 0.95:
                        risk_level = "high"
                        
                    finding = {
                        "case_id": case_id,
                        "job_id": job_id,
                        "worker_type": "audio",
                        "category": "sentiment",
                        "confidence": float(score),
                        "risk_level": risk_level,
                        "evidence": {
                            "transcription": text,
                            "timestamp": timestamp,
                            "sentiment": label,
                            "score": float(score)
                        }
                    }
                    findings.append(finding)
                    submit_finding_fn(finding)
            except Exception as e:
                logger.warning(f"Sentiment analysis failed on audio segment '{text[:50]}...': {e}")
                
    report_progress_fn(job_id, 100, "completed")
    return findings
