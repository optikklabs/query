                                                                       
                                                            

ALTER TABLE optikk.users
  ADD COLUMN IF NOT EXISTS terms_accepted_at DATETIME NULL,
  ADD COLUMN IF NOT EXISTS terms_version VARCHAR(20) NOT NULL DEFAULT '';
