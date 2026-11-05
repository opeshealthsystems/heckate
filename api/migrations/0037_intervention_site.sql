-- Where a site visit takes place and whom to meet: shown to the engineer and opened in Maps by the mobile app.
ALTER TABLE projects.interventions
  ADD COLUMN site_address text CHECK (length(site_address) <= 500),
  ADD COLUMN site_contact text CHECK (length(site_contact) <= 200);
