-- ============================================
-- File: 000048_triage_vitals_questions.sql
-- ============================================
-- Adds the patient-vitals fields that were added to the incident triage form
-- (respiratory rate, SpO2, pulse, blood pressure, temperature) as questions on
-- the EMS_PRIMARY_TRIAGE questionnaire so they are returned by
-- GET /api/v1/reference/triage-questions.
--
-- They are seeded with response_type = TEXT on purpose. The triage scoring
-- engine (incidents service PersistTriageSession) applies a fixed threshold
-- score to EVERY INTEGER question (n>=5 -> 90, n>=3 -> 50, n>=1 -> 10). A pulse
-- of 82 or a respiratory rate of 18 would therefore score 90 and force every
-- incident to RED. TEXT responses are stored verbatim and carry no score, which
-- is the correct behaviour for raw vitals (a value like "120/80" or "37.2" also
-- isn't a plain integer). They are not required because vitals are frequently
-- unavailable at phone/community triage.

-- +goose Up
INSERT INTO triage_questions (questionnaire_id, code, question_text, response_type, display_order, is_required)
SELECT tq.id, v.code, v.question_text, v.response_type, v.display_order, FALSE
FROM triage_questionnaires tq
JOIN (
    VALUES
        ('RESPIRATORY_RATE', 'What is the patient''s respiratory rate (breaths per minute)?', 'TEXT', 6),
        ('OXYGEN_SATURATION', 'What is the patient''s oxygen saturation / SpO2 (%)?', 'TEXT', 7),
        ('PULSE_RATE', 'What is the patient''s pulse rate (bpm)?', 'TEXT', 8),
        ('BLOOD_PRESSURE', 'What is the patient''s blood pressure (systolic/diastolic)?', 'TEXT', 9),
        ('BODY_TEMPERATURE', 'What is the patient''s body temperature (°C)?', 'TEXT', 10)
) AS v(code, question_text, response_type, display_order)
ON tq.code = 'EMS_PRIMARY_TRIAGE'
ON CONFLICT (code) DO NOTHING;

-- +goose Down
DELETE FROM triage_questions
WHERE code IN (
    'RESPIRATORY_RATE',
    'OXYGEN_SATURATION',
    'PULSE_RATE',
    'BLOOD_PRESSURE',
    'BODY_TEMPERATURE'
)
AND questionnaire_id IN (
    SELECT id FROM triage_questionnaires WHERE code = 'EMS_PRIMARY_TRIAGE'
);
