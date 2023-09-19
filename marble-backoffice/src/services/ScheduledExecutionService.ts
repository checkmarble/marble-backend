import { type LoadingDispatcher } from "@/hooks/Loading";
import { useSimpleLoader } from "@/hooks/SimpleLoader";
import { type ScheduleExecution } from "@/models";
import {
  ScheduledExecutionRepository,
  fetchScheduledExecutions,
} from "@/repositories";
import { useCallback } from "react";

export interface ScheduledExecutionService {
  scheduledExecutionRepository: ScheduledExecutionRepository;
}

export function useScheduledExecutions({
  service,
  organizationId,
  loadingDispatcher,
}: {
  service: ScheduledExecutionService;
  organizationId: string;
  loadingDispatcher: LoadingDispatcher;
}) {
  const scheduledExecutionLoader = useCallback(async (): Promise<
    ScheduleExecution[] | null
  > => {
    return fetchScheduledExecutions(
      service.scheduledExecutionRepository,
      organizationId
    );
  }, [organizationId, service.scheduledExecutionRepository]);

  const [scheduledExecutions] = useSimpleLoader(
    loadingDispatcher,
    scheduledExecutionLoader
  );

  return {
    scheduledExecutions,
  };
}

export async function downloadScheduledExecutionsDecisionsLink({
  service,
  organizationId,
  scheduleExecutionId,
}: {
  service: ScheduledExecutionService;
  organizationId: string;
  scheduleExecutionId: string;
}): Promise<string> {
  const file =
    await service.scheduledExecutionRepository.marbleApi.decisionsOfScheduleExecution(
      {
        organizationId,
        scheduleExecutionId,
      }
    );

  return URL.createObjectURL(file);
}
