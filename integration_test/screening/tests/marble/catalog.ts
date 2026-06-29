import fs from "fs";

import {
	PutBucketPolicyCommand,
	PutObjectCommand,
	S3Client,
} from "@aws-sdk/client-s3";
import { StartedNetwork, StartedTestContainer } from "testcontainers";
import { uri } from "./utils";

export const createFakeCatalog = async (
	network: StartedNetwork,
	s3: StartedTestContainer,
) => {
	await putBucketPolicy(network, s3);
	await uploadCatalog(network, s3);
};

let firstVersion: string;

const uploadCatalog = async (
	network: StartedNetwork,
	s3: StartedTestContainer,
) => {
	firstVersion = createDatasetVersion();

	const entities = [
		{
			id: "Q30343128",
			schema: "Person",
			properties: { name: ["Mathilde Panot"] },
		},
		{
			id: "Q2960570",
			schema: "Person",
			properties: { name: ["Charles de Courson"] },
		},
	];

	const s3c = client(network, s3);

	await s3c.send(
		new PutObjectCommand({
			Bucket: "marble",
			Key: "fake/index.json",
			Body: JSON.stringify(createIndex(firstVersion)),
		}),
	);

	await s3c.send(
		new PutObjectCommand({
			Bucket: "marble",
			Key: `fake/${firstVersion}/entities.ftm.json`,
			Body: entities.map((line) => JSON.stringify(line)).join("\n"),
		}),
	);
};

export const uploadDelta = async (
	network: StartedNetwork,
	s3: StartedTestContainer,
) => {
	const newVersion = createDatasetVersion();

	const delta = [
		{
			op: "ADD",
			entity: {
				id: "apognu",
				schema: "Person",
				datasets: ["fr_assemblee"],
				caption: "Céline Hervieu",
				properties: { name: ["Céline Hervieu"] },
			},
		},
	];

	const s3c = client(network, s3);

	await s3c.send(
		new PutObjectCommand({
			Bucket: "marble",
			Key: `fake/${newVersion}/entities.delta.json`,
			Body: delta.map((line) => JSON.stringify(line)).join("\n"),
		}),
	);

	const deltas = {
		versions: {
			[firstVersion]: "",
			[newVersion]: `http://s3:7070/marble/fake/${newVersion}/entities.delta.json`,
		},
	};

	await s3c.send(
		new PutObjectCommand({
			Bucket: "marble",
			Key: `fake/${newVersion}/deltas.json`,
			Body: JSON.stringify(deltas),
		}),
	);

	await s3c.send(
		new PutObjectCommand({
			Bucket: "marble",
			Key: "fake/index.json",
			Body: JSON.stringify(createIndex(newVersion)),
		}),
	);
};

const putBucketPolicy = async (
	network: StartedNetwork,
	s3: StartedTestContainer,
) => {
	const s3c = client(network, s3);

	await s3c.send(
		new PutBucketPolicyCommand({
			Bucket: "marble",
			Policy: JSON.stringify({
				Version: "2012-10-17",
				Statement: [
					{
						Action: ["s3:GetObject", "s3:GetBucketLocation"],
						Effect: "Allow",
						Principal: { AWS: ["*"] },
						Resource: ["arn:aws:s3:::marble", "arn:aws:s3:::marble/*"],
						Sid: "",
					},
				],
			}),
		}),
	);
};

const client = (network: StartedNetwork, s3: StartedTestContainer) => {
	return new S3Client({
		endpoint: uri(network, s3, 7070),
		region: "us-east-1",
		credentials: { accessKeyId: "root", secretAccessKey: "azertyuiop" },
	});
};

const createIndex = (version: string) => {
	return {
		datasets: [
			{
				name: "default",
				title: "Default",
				type: "collection",
				version,
				children: ["fr_assemblee"],
				datasets: ["fr_assemblee"],
			},
			{
				name: "fr_assemblee",
				title: "Assemblée Nationale",
				type: "external",
				version,
				url: "https://www.checkmarble.com",
				entities_url: `http://s3:7070/marble/fake/${version}/entities.ftm.json`,
				delta_url: `http://s3:7070/marble/fake/${version}/deltas.json`,
				resources: [
					{
						name: "entitites.ftm.json",
						url: `http://s3:7070/marble/fake/${version}/entities.ftm.json`,
						mime_type: "application/json+ftm",
						mime_type_label: "FollowTheMoney Entities",
						path: "entities.ftm.json",
						size: 0,
						checksum: "",
					},
				],
			},
		],
	};
};

const createDatasetVersion = () => {
	const now = new Date();

	const year = now.getFullYear();
	const month = String(now.getMonth() + 1).padStart(2, "0");
	const day = String(now.getDate()).padStart(2, "0");
	const hours = String(now.getHours()).padStart(2, "0");
	const minutes = String(now.getMinutes()).padStart(2, "0");
	const seconds = String(now.getSeconds()).padStart(2, "0");

	return `${year}${month}${day}${hours}${minutes}${seconds}-mar`;
};
