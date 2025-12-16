-- ============================================
-- RATING SYSTEM MIGRATION: 10-Star → 5-Star
-- ============================================
-- WARNING: This script modifies user rating data!
-- ALWAYS backup your database before running!
-- ============================================
-- Step 1: Create backup table
CREATE TABLE IF NOT EXISTS user_progress_backup_10star AS
SELECT *
FROM user_progress;
-- Step 2: Display current rating distribution (BEFORE)
SELECT 'BEFORE MIGRATION' as phase,
    '═══════════════════════' as separator;
SELECT user_rating as original_rating,
    COUNT(*) as count,
    ROUND(
        COUNT(*) * 100.0 / (
            SELECT COUNT(*)
            FROM user_progress
            WHERE user_rating IS NOT NULL
        ),
        1
    ) as percentage
FROM user_progress
WHERE user_rating IS NOT NULL
GROUP BY user_rating
ORDER BY user_rating DESC;
-- Step 3: Convert ratings using formula: new_rating = CEIL(old_rating / 2)
UPDATE user_progress
SET user_rating = CAST(CEIL(user_rating / 2.0) AS REAL)
WHERE user_rating IS NOT NULL;
-- Step 4: Display new rating distribution (AFTER)
SELECT 'AFTER MIGRATION' as phase,
    '═══════════════════════' as separator;
SELECT user_rating as new_rating,
    COUNT(*) as count,
    ROUND(
        COUNT(*) * 100.0 / (
            SELECT COUNT(*)
            FROM user_progress
            WHERE user_rating IS NOT NULL
        ),
        1
    ) as percentage
FROM user_progress
WHERE user_rating IS NOT NULL
GROUP BY user_rating
ORDER BY user_rating DESC;
-- Step 5: Validation check
SELECT 'VALIDATION CHECK' as phase,
    '═══════════════════════' as separator;
SELECT CASE
        WHEN COUNT(*) = 0 THEN '✅ All ratings are within 1-5 range'
        ELSE '❌ ERROR: Found ' || COUNT(*) || ' ratings outside 1-5 range!'
    END as status
FROM user_progress
WHERE user_rating IS NOT NULL
    AND (
        user_rating < 1
        OR user_rating > 5
    );
-- Step 6: Summary statistics
SELECT 'SUMMARY' as phase,
    '═══════════════════════' as separator;
SELECT COUNT(*) as total_ratings,
    ROUND(AVG(user_rating), 2) as new_average_rating,
    MIN(user_rating) as min_rating,
    MAX(user_rating) as max_rating
FROM user_progress
WHERE user_rating IS NOT NULL;
-- ============================================
-- ROLLBACK (if needed)
-- ============================================
-- Uncomment to restore original ratings:
-- DROP TABLE user_progress;
-- ALTER TABLE user_progress_backup_10star RENAME TO user_progress;
-- ============================================