
export interface ScheduleExecution {
    id: string;
    scenario_iteration_id: string;
    status: string;
    started_at: Date;
    finished_at: Date | null;
}
