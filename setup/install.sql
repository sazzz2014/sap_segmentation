BEGIN;
CREATE TABLE IF NOT EXISTS segmentation (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    address_sap_id VARCHAR(255) NOT NULL,
    adr_segment VARCHAR(16) NOT NULL,
    segment_id BIGINT NOT NULL,
    CONSTRAINT segmentation_address_sap_id_key UNIQUE (address_sap_id)
);
COMMIT;
