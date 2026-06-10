import psycopg2
from psycopg2.extras import RealDictCursor
import json
from datetime import datetime

def get_db_connection():
    return psycopg2.connect(
        host="postgres",  
        port=5432,
        dbname="mediaplatform",
        user="media",
        password="media"
    )

def calculate_risk_score(findings):
    if not findings:
        return 0.0
    
    # Pesos por nivel de riesgo
    risk_weights = {'critical': 10, 'high': 7, 'medium': 4, 'low': 1}
    
    total_score = 0
    for f in findings:
        weight = risk_weights.get(f['risk_level'], 1)
        # Promedio ponderado por la confianza del modelo
        total_score += weight * f['confidence']
        
    avg_score = total_score / len(findings)
    # Escalar para que el máximo sea 10
    return min(round(avg_score, 1), 10.0)

def generate_report(case_id: str) -> dict:
    conn = get_db_connection()
    cursor = conn.cursor(cursor_factory=RealDictCursor)
    
    try:
        # Extraer hallazgos del caso
        cursor.execute("""
            SELECT id, worker_type, category, confidence, risk_level, evidence, created_at
            FROM findings
            WHERE case_id = %s
            ORDER BY created_at ASC
        """, (case_id,))
        findings = cursor.fetchall()

        findings_by_type = {'text': [], 'image': [], 'audio': []}
        timeline = []

        for f in findings:
            finding_dict = dict(f)
            # Asegurar que evidence sea un dict
            if isinstance(finding_dict['evidence'], str):
                finding_dict['evidence'] = json.loads(finding_dict['evidence'])
            
            # Formatear fecha
            finding_dict['created_at'] = finding_dict['created_at'].isoformat()
            
            findings_by_type[f['worker_type']].append(finding_dict)
            timeline.append(finding_dict)

        risk_score = calculate_risk_score(findings)

        report = {
            "case_id": case_id,
            "generated_at": datetime.now().isoformat(),
            "risk_score": risk_score,
            "total_findings": len(findings),
            "findings_by_type": findings_by_type,
            "timeline": timeline,
            "recommendations": ["Revisión manual requerida"] if risk_score > 7 else ["Monitoreo estándar"]
        }
        
        return report
        
    except Exception as e:
        return {"error": str(e)}
    finally:
        cursor.close()
        conn.close()

if __name__ == "__main__":
    import sys
    if len(sys.argv) > 1:
        case_id = sys.argv[1]
        print(json.dumps(generate_report(case_id), indent=2))
    else:
        print(json.dumps({"error": "No case_id provided"}))