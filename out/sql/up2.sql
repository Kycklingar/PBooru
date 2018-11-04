CREATE OR REPLACE FUNCTION update_comic_order()
RETURNS trigger AS $$
BEGIN
    UPDATE comics
    SET modified = CURRENT_TIMESTAMP
    WHERE id = NEW.comic_id;
    RETURN NEW;
END;
$$
LANGUAGE 'plpgsql';

CREATE TRIGGER "comic_modified_tr"
AFTER INSERT ON "comic_mappings"
FOR EACH ROW
EXECUTE PROCEDURE update_comic_order();

CREATE FUNCTION update_tag_count()
RETURNS trigger AS $$
BEGIN
    UPDATE tags
    SET count=(
        SELECT count(1)
        FROM post_tag_mappings
        WHERE tag_id = NEW.tag_id
    )
    WHERE id = NEW.tag_id;
    RETURN NEW;
END;
$$
LANGUAGE 'plpgsql';

CREATE FUNCTION update_tag_count_del()
RETURNS trigger AS $$
BEGIN
    UPDATE tags
    SET count=(
        SELECT count(1)
        FROM post_tag_mappings
        WHERE tag_id = OLD.tag_id
    )
    WHERE id = OLD.tag_id;
    RETURN NEW;
END;
$$
LANGUAGE 'plpgsql';

CREATE TRIGGER "tags_count_trigger"
AFTER INSERT ON "post_tag_mappings"
FOR EACH ROW
EXECUTE PROCEDURE update_tag_count();

CREATE TRIGGER "tags_count_trigger_del"
AFTER DELETE ON "post_tag_mappings"
FOR EACH ROW
EXECUTE PROCEDURE update_tag_count_del();
