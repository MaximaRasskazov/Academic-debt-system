-- Участники пересдачи: студенты, обычные преподаватели, члены комиссии.
--
-- Единая таблица под все типы участников — поле kind различает их.
-- Это упрощает запросы "все участники пересдачи X" одним SELECT, а
-- не UNION'ом трёх таблиц.
--
-- kind:
--   student            — сдающий, обязательно ссылается на debt_id
--                        (его долг закрывается оценкой на этой пересдаче);
--   teacher            — преподаватель в обычной пересдаче;
--   commission_member  — член комиссии (при kind='commission' у retakes).
--
-- grade выставляется преподавателем после пересдачи, только для
-- студентов. Когда grade проставлен, в той же транзакции сервис
-- обновляет связанный debts.status = 'graded' и debts.final_grade,
-- чтобы данные были консистентны (с источником правды в debts).
-- Здесь grade хранится отдельно, чтобы видеть результат КАЖДОЙ
-- попытки пересдачи — это даёт историю "первая попытка 2, вторая 4".

-- +goose Up
CREATE TABLE IF NOT EXISTS retake_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    retake_id UUID NOT NULL REFERENCES retakes(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    kind VARCHAR(20) NOT NULL CHECK (kind IN ('student', 'teacher', 'commission_member')),
    debt_id UUID REFERENCES debts(id) ON DELETE RESTRICT,
    grade INTEGER CHECK (grade IS NULL OR grade BETWEEN 2 AND 5),
    graded_at TIMESTAMP WITH TIME ZONE,
    graded_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Студент сдаёт что-то конкретное (debt_id обязателен).
    -- Преподаватели/комиссия — без debt_id.
    CONSTRAINT participants_student_has_debt CHECK (
        (kind = 'student' AND debt_id IS NOT NULL)
        OR (kind IN ('teacher', 'commission_member') AND debt_id IS NULL)
    ),
    -- Оценку выставляют только студентам.
    CONSTRAINT participants_grade_only_for_students CHECK (
        kind = 'student' OR (grade IS NULL AND graded_at IS NULL AND graded_by IS NULL)
    ),
    -- Если grade проставлен — graded_at/by тоже должны быть.
    CONSTRAINT participants_grade_consistency CHECK (
        (grade IS NULL AND graded_at IS NULL AND graded_by IS NULL)
        OR (grade IS NOT NULL AND graded_at IS NOT NULL AND graded_by IS NOT NULL)
    )
);

-- Один и тот же пользователь не может быть включён в пересдачу дважды
-- (например, и студентом, и преподавателем — это абсурдно).
CREATE UNIQUE INDEX IF NOT EXISTS idx_retake_participants_unique ON retake_participants (retake_id, user_id);
CREATE INDEX IF NOT EXISTS idx_retake_participants_retake ON retake_participants (retake_id);
CREATE INDEX IF NOT EXISTS idx_retake_participants_user ON retake_participants (user_id);
CREATE INDEX IF NOT EXISTS idx_retake_participants_debt ON retake_participants (debt_id) WHERE debt_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_retake_participants_kind ON retake_participants (kind);

-- +goose Down
DROP TABLE IF EXISTS retake_participants;
