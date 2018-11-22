CREATE OR REPLACE FUNCTION alias_insert()
RETURNS trigger AS $$
BEGIN
    -- Update all the paren_tags to reflect the new alias
    -- Have to use annoying selects because there is no 
    -- IGNORE on updates in postgresql
    UPDATE parent_tags 
    SET parent_id = NEW.alias_to
    WHERE parent_id = NEW.alias_from
    AND (
        SELECT count(*)
        FROM parent_tags
        WHERE parent_id = NEW.alias_to
        ) <= 0;

    DELETE FROM parent_tags
    WHERE parent_id = NEW.alias_from;

    UPDATE parent_tags
    SET child_id = NEW.alias_to
    WHERE child_id = NEW.alias_from
    AND (
        SELECT count(*)
        FROM parent_tags
        WHERE child_id = NEW.alias_to
    ) <= 0;

    DELETE FROM parent_tags
    WHERE child_id = NEW.alias_from;

    -- Update old alias to use the new alias
    -- Could probably use some checking like above
    UPDATE alias
    SET alias_to = NEW.alias_to
    WHERE alias_to = NEW.alias_from;
    
    -- Update posts where the new alias have a parent tag
    INSERT INTO post_tag_mappings (post_id, tag_id)
    
        (SELECT post_id, (
            SELECT parent_id
            FROM parent_tags
            WHERE child_id = NEW.alias_to
            )
        FROM post_tag_mappings
        WHERE tag_id = NEW.alias_from
        AND (
            SELECT parent_id 
            FROM parent_tags 
            WHERE child_id = NEW.alias_to
            ) IS NOT NULL
        )
     ON CONFLICT DO NOTHING;

    -- Insert the new alias and delete the old tag from posts
    -- This is just an update with checks because postgresql
    -- doesn't have ignores
    INSERT INTO post_tag_mappings (post_id, tag_id)
        SELECT post_id, NEW.alias_to
        FROM post_tag_mappings
        WHERE tag_id = NEW.alias_from
    ON CONFLICT DO NOTHING;

    DELETE FROM post_tag_mappings
    WHERE tag_id = NEW.alias_from;

    RETURN NEW;
END;
$$
LANGUAGE 'plpgsql';

DROP TRIGGER IF EXISTS "alias_insert_trigger" ON alias;

CREATE TRIGGER "alias_insert_trigger"
AFTER INSERT ON "alias"
FOR EACH ROW
EXECUTE PROCEDURE alias_insert();

DROP TRIGGER IF EXISTS "alias_update_trigger" ON alias;

CREATE TRIGGER "alias_update_trigger"
AFTER UPDATE ON "alias"
FOR EACH ROW
EXECUTE PROCEDURE alias_insert();