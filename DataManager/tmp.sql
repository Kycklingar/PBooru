SELECT count(t1.post_id)
FROM post_tag_mappings t1 
JOIN post_tag_mappings t2 
    ON t1.post_id = t2.post_id 
    AND t2.tag_id = 4
AND t1.tag_id = 2
AND t1.post_id NOT IN(SELECT post_id FROM post_tag_mappings WHERE tag_id = 209)
AND (
    SELECT deleted 
    FROM posts 
    WHERE id = t1.post_id
    ) = 0;

EXPLAIN
SELECT id FROM posts WHERE id IN(SELECT t1.post_id
FROM post_tag_mappings t1 
JOIN post_tag_mappings t2 
    ON t1.post_id = t2.post_id 
    AND t2.tag_id = 4
AND t1.post_id NOT IN(SELECT post_id FROM post_tag_mappings WHERE tag_id = 209)
AND t1.tag_id = 2
AND (
    SELECT deleted
    FROM posts 
    WHERE id = t1.post_id
    ) = 0
    LIMIT 10
);

SELECT id FROM posts WHERE id IN(
    SELECT t1.post_id
    FROM post_tag_mappings t1 
    JOIN post_tag_mappings t2 
    ON t1.post_id = t2.post_id 
    AND t2.tag_id = 4
    AND t1.post_id NOT IN(
        SELECT post_id
        FROM post_tag_mappings
        WHERE tag_id = 209
        )
    AND t1.tag_id = 2
    AND (
        SELECT deleted
        FROM posts 
        WHERE id = t1.post_id
    ) = 0
    ORDER BY t1.post_id DESC
    LIMIT 10
);


SELECT id, (SELECT nspace FROM namespaces WHERE id = t.namespace_id), tag FROM tags t WHERE tag = 'krystal';




SELECT id, multihash, thumbhash, mime_id 
FROM posts 
WHERE id IN
(
    SELECT t1.post_id 
    FROM post_tag_mappings t1
    JOIN post_tag_mappings t2 
    ON t1.post_id = t2.post_id 
    AND t2.tag_id = 2 
    AND t1.tag_id = 4 
    AND t1.post_id NOT IN
    (
        SELECT post_id 
        FROM post_tag_mappings 
        WHERE 
        (
            tag_id = 6 
            OR tag_id = 224
        ) 
        AND post_id NOT IN
        (
            SELECT post_id 
            FROM post_tag_mappings 
            WHERE  tag_id = 2689 
            OR tag_id = 4692
        )
    )
    AND 
    (
        SELECT deleted 
        FROM posts 
        WHERE id = t1.post_id
    ) = false
)
ORDER BY id DESC 
LIMIT 500 OFFSET 0



SELECT p1.id 
FROM posts p1 
JOIN post_tag_mappings t1 
ON t1.post_id = p1.id

    JOIN post_tag_mappings t2 
    ON t1.post_id = t2.post_id 
    AND t2.tag_id = 2 
    AND t1.tag_id = 4 

    EXCEPT
    (
		SELECT post_id
    	FROM post_tag_mappings
    	WHERE 
        tag_id = 6
        OR tag_id = 224
		
		EXCEPT
		(
			SELECT post_id
			FROM post_tag_mappings
			WHERE tag_id = 2689
			OR tag_id = 4692
		)
    )
	ORDER BY id DESC
    LIMIT 500 OFFSET 0


SELECT id FROM posts p1
JOIN post_tag_mappings t1
ON p1.id = t1.post_id
JOIN post_tag_mappings t2
ON t1.post_id = t2.post_id
FULL OUTER JOIN post_tag_mappings f1
ON t1.post_id = f1.post_id 
AND (
	f1.tag_id = 6 
	OR f1.tag_id = 224
)
FULL OUTER JOIN post_tag_mappings u1
ON t1.post_id = u1.post_id
AND (
	u1.tag_id = 4692
	OR u1.tag_id = 2689
	)

WHERE f1.post_id IS NULL
AND u1.post_id IS NULL
AND t1.tag_id = 4
AND t2.tag_id = 2
AND p1.deleted = false
		   
ORDER BY t1.post_id DESC 
LIMIT 500 OFFSET 5000

SELECT id, multihash, thumbhash, mime_id 
FROM posts 
WHERE id IN(
    SELECT id 
    FROM posts p1 
    JOIN post_tag_mappings t1 
    ON t1.post_id = p1.id 
    AND p1.deleted = false 
    JOIN post_tag_mappings t2 
    ON t1.post_id = t2.post_id 
    FULL OUTER JOIN post_tag_mappings f1 
    ON t1.post_id = f1.post_id 
    AND( 
        f1.tag_id = 6 
        OR f1.tag_id = 209
        )  
    LEFT JOIN post_tag_mappings u1 
    ON t1.post_id = u1.post_id 
    AND( 
        u1.tag_id = 2689 
        OR u1.tag_id = 4692
        )
    WHERE u1.post_id IS NULL 
    AND f1.post_id IS NULL 
    AND t2.tag_id = 2 
    AND t1.tag_id = 4  
    ORDER BY id DESC LIMIT $1 OFFSET $2)



SELECT count(*)
FROM post_tag_mappings t1
FULL OUTER JOIN post_tag_mappings f1
ON t1.post_id = f1.post_id
AND (f1.tag_id = 6 OR f1.tag_id = 209 )
LEFT OUTER JOIN post_tag_mappings u1
ON t1.post_id = u1.post_id
AND (u1.tag_id = 4692 OR u1.tag_id = 2689)
WHERE t1.tag_id = 4
AND (f1.post_id IS NULL OR u1.post_id IS NOT NULL)
--ORDER BY t1.post_id DESC
--LIMIT 500 OFFSET 0

 -- FALSE	1 | 0 | 0
 -- TRUE	1 | 0 | 1
 -- TRUE	1 | 1 | 0
 -- TRUE	1 | 1 | 1

SELECT count(*) 
FROM post_tag_mappings t1  
FULL OUTER JOIN post_tag_mappings f1 
ON t1.post_id = f1.post_id 
AND (f1.tag_id = 6 OR f1.tag_id = 209)
LEFT JOIN post_tag_mappings u1 
ON t1.post_id = u1.post_id 
AND (u1.tag_id = 2689 OR u1.tag_id = 4692)
WHERE (u1.post_id IS NOT NULL OR f1.post_id IS NULL)
AND t1.tag_id = 4 
