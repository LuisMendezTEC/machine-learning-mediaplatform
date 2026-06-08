from pydantic import BaseModel
from typing import List, Dict, Any, Optional

class BoundingBox(BaseModel):
    x: int
    y: int
    w: int
    h: int

class Evidence(BaseModel):
    text_fragment: Optional[str] = None
    timestamp: Optional[str] = None
    keywords: Optional[List[str]] = None
    bounding_box: Optional[BoundingBox] = None
    transcription: Optional[str] = None

class Finding(BaseModel):
    job_id: str
    case_id: str
    worker_type: str  # "text" | "image" | "audio"
    category: str     # "keyword" | "sentiment" | "violence" | "offensive" | "weapon" | "threat"
    confidence: float # 0.0 - 1.0
    risk_level: str   # "low" | "medium" | "high" | "critical"
    evidence: Evidence
