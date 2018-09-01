ALTER TABLE tags ADD COLUMN count INTEGER NOT NULL DEFAULT 0;

CREATE TRIGGER tags_count_trigger 
AFTER INSERT ON post_tag_mappings 
BEGIN 
    UPDATE tags 
    SET count=(
            SELECT count() 
            FROM post_tag_mappings 
            WHERE tag_id=id
        ) 
        WHERE id=NEW.tag_id; 
END;

CREATE TRIGGER tags_count_trigger_del 
AFTER DELETE ON post_tag_mappings 
BEGIN 
    UPDATE tags 
    SET count=(
            SELECT count() 
            FROM post_tag_mappings 
            WHERE tag_id=id
        ) 
        WHERE id=OLD.tag_id; 
END;

CREATE TRIGGER tags_count_trigger_update 
AFTER UPDATE ON post_tag_mappings 
BEGIN 
    UPDATE tags 
    SET count=(
            SELECT count() 
            FROM post_tag_mappings 
            WHERE tag_id=id
        ) 
        WHERE id=NEW.tag_id OR id=OLD.tag_id;
END;

UPDATE tags
SET count=
(
	SELECT count() 
    FROM post_tag_mappings 
    WHERE tag_id=id
);