import os
import json
import logging
import spacy
from transformers import pipeline

logger = logging.getLogger("text_analyzer")

# Initialize models globally (cached)
try:
    logger.info("Loading spacy models...")
    nlp_es = spacy.load("es_core_news_sm")
    nlp_en = spacy.load("en_core_web_sm")
except Exception as e:
    logger.warning(f"Could not load spacy models dynamically, will attempt fallback load: {e}")
    nlp_es = None
    nlp_en = None

try:
    logger.info("Loading HuggingFace sentiment pipeline...")
    # Use a lightweight multilingual sentiment model
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
    "gun", "knife", "violence", "attack", "death", "murder", "rape", "threat", "amenaza"
]

def analyze_text(file_path: str, job_id: str, case_id: str, submit_finding_fn, report_progress_fn) -> list:
    logger.info(f"Analyzing text file: {file_path}")
    
    # 1. Read file
    if not os.path.exists(file_path):
        raise FileNotFoundError(f"File not found: {file_path}")
        
    with open(file_path, "r", encoding="utf-8", errors="ignore") as f:
        content = f.read()
        
    sentences_to_process = []
    # Try parsing as JSON first (chat export)
    try:
        data = json.loads(content)
        # Check if it has a list of messages
        if isinstance(data, list):
            for msg in data:
                if isinstance(msg, dict) and "text" in msg:
                    sentences_to_process.append(str(msg["text"]))
                elif isinstance(msg, str):
                    sentences_to_process.append(msg)
        elif isinstance(data, dict):
            if "messages" in data and isinstance(data["messages"], list):
                for msg in data["messages"]:
                    if isinstance(msg, dict) and "text" in msg:
                        sentences_to_process.append(str(msg["text"]))
            elif "text" in data:
                sentences_to_process.append(str(data["text"]))
            else:
                # Fallback: dump string representations of values
                for v in data.values():
                    if isinstance(v, str):
                        sentences_to_process.append(v)
    except json.JSONDecodeError:
        # It's plain text
        pass
        
    # If it's plain text, we use spaCy to split into sentences
    if not sentences_to_process and content:
        # Simple heuristic to choose between English and Spanish spaCy
        spanish_words = {'el', 'la', 'los', 'las', 'un', 'una', 'y', 'o', 'que', 'en', 'de', 'para', 'con'}
        words = set(content.lower().split()[:100])
        nlp = nlp_es if (words.intersection(spanish_words) and nlp_es) else (nlp_en or nlp_es)
        
        if nlp:
            doc = nlp(content)
            sentences_to_process = [sent.text.strip() for sent in doc.sents if sent.text.strip()]
        else:
            # Fallback split by lines
            sentences_to_process = [s.strip() for s in content.split("\n") if s.strip()]

    total_sentences = len(sentences_to_process)
    logger.info(f"Total sentences/messages to process: {total_sentences}")
    
    if total_sentences == 0:
        return []
        
    findings = []
    
    for idx, sentence in enumerate(sentences_to_process):
        # Progress reporting (every 10% or at the end)
        progress_pct = int(((idx + 1) / total_sentences) * 100)
        if progress_pct % 10 == 0 or idx == total_sentences - 1:
            report_progress_fn(job_id, progress_pct, "running")
            
        sentence_lower = sentence.lower()
        
        # 2. Check risk keywords
        matched_keywords = [kw for kw in RISK_KEYWORDS if kw in sentence_lower]
        if matched_keywords:
            risk_level = "medium"
            category = "keyword"
            
            # Map categories/risk based on keywords
            weapons = ["cuchillo", "pistola", "arma", "knife", "gun", "weapon"]
            violence = ["matar", "asesinar", "violación", "violencia", "kill", "murder", "rape", "violence", "threat", "amenaza", "secuestro", "kidnap"]
            
            if any(w in matched_keywords for w in weapons):
                category = "weapon"
                risk_level = "high"
            elif any(v in matched_keywords for v in violence):
                category = "violence"
                risk_level = "high"
                
            finding = {
                "case_id": case_id,
                "job_id": job_id,
                "worker_type": "text",
                "category": category,
                "confidence": 1.0,
                "risk_level": risk_level,
                "evidence": {
                    "text_fragment": sentence,
                    "keywords": matched_keywords
                }
            }
            findings.append(finding)
            submit_finding_fn(finding)
            
        # 3. Sentiment analysis
        if sentiment_pipeline and len(sentence.strip()) > 3:
            try:
                # Limit sentence length to avoid model limits
                truncated_sentence = sentence[:400]
                result = sentiment_pipeline(truncated_sentence)[0]
                label = result["label"].lower() # "positive", "neutral", "negative"
                score = result["score"]
                
                if label == "negative" and score > 0.65:
                    risk_level = "low"
                    if score > 0.85:
                        risk_level = "medium"
                    if score > 0.95:
                        risk_level = "high"
                        
                    finding = {
                        "case_id": case_id,
                        "job_id": job_id,
                        "worker_type": "text",
                        "category": "sentiment",
                        "confidence": float(score),
                        "risk_level": risk_level,
                        "evidence": {
                            "text_fragment": sentence,
                            "sentiment": label,
                            "score": float(score)
                        }
                    }
                    findings.append(finding)
                    submit_finding_fn(finding)
            except Exception as e:
                logger.warning(f"Sentiment analysis failed on sentence '{sentence[:50]}...': {e}")
                
    return findings
