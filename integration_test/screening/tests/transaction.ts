import { API_KEY } from "./marble/setup";

export const testTransactionMonitoring = async (
	apiUrl: string,
	scenarioId: string,
) => {
	interface Decision {
		name: string;
		outcome: "approve" | "decline" | "review" | "block_and_review";
		status: "no_hit" | "in_review";
		entity_id?: string;
		score?: number;
	}

	const tt: Decision[] = [
		{
			name: "Mathilde Panot",
			outcome: "block_and_review",
			status: "in_review",
			entity_id: "Q30343128",
			score: 1.0,
		},
		{ name: "Vladimir Putin", outcome: "approve", status: "no_hit" },
	];

	for (const spec of tt) {
		let decisionResp = await fetch(`${apiUrl}/v1/decisions`, {
			method: "POST",
			headers: { "x-api-key": API_KEY },
			body: JSON.stringify({
				scenario_id: scenarioId,
				trigger_object: {
					object_id: "anything",
					updated_at: new Date().toISOString(),
					name: spec.name,
				},
			}),
		});

		expect(decisionResp.status).toBe(200);

		let decision = await decisionResp.json();

		expect(decision.data).toHaveLength(1);
		expect(decision.data[0].outcome).toBe(spec.outcome);
		expect(decision.data[0].screenings).toHaveLength(1);
		expect(decision.data[0].screenings[0].status).toBe(spec.status);

		if (spec.status == "in_review" && spec.entity_id && spec.score) {
			let screeningsResp = await fetch(
				`${apiUrl}/v1/decisions/${decision.data[0].id}/screenings`,
				{
					headers: { "x-api-key": API_KEY },
				},
			);

			expect(screeningsResp.status).toBe(200);

			let screenings = await screeningsResp.json();

			expect(screenings.data[0].matches[0].payload.id).toBe(spec.entity_id);
			expect(screenings.data[0].matches[0].payload.score).toBeCloseTo(
				spec.score,
			);
		}
	}
};
