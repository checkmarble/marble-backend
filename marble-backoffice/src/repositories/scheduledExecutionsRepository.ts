import type { MarbleApi } from "@/infra/MarbleApi";
import type { ScheduleExecution } from "@/models";
import { adaptScheduleExecutionApiResult } from "@/models/ScheduleExecutionDto";

export interface ScheduledExecutionsRepository {
  marbleApi: MarbleApi;
}

export async function fetchScheduledExecutions(
  repository: ScheduledExecutionsRepository,
  organizationId: string
): Promise<ScheduleExecution[]> {
  return adaptScheduleExecutionApiResult(
    await repository.marbleApi.scheduleExecutionOfOrganization({
      organizationId,
    })
  );
}
