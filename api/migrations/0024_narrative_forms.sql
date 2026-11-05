-- More kinds of text document: the numbered HS-FRM forms, guarantee policies, contracts, penetration test reports.
ALTER TABLE documents.narratives DROP CONSTRAINT narratives_kind_check;
ALTER TABLE documents.narratives ADD CONSTRAINT narratives_kind_check
  CHECK (kind IN ('proposal','order_confirmation','sow','letter','certificate','form','guarantee','contract','pentest_report'));
ALTER TABLE documents.narratives ADD COLUMN form_code text CHECK (form_code ~ '^FRM-[0-9]{2}$');
ALTER TABLE documents.narratives ADD CONSTRAINT narratives_form_code CHECK ((kind = 'form') = (form_code IS NOT NULL));
