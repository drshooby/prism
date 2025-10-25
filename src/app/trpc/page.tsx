"use client";

import { api } from "@/trpc/react";

export default function TRPCPage() {
  const { data } = api.github.getUserRepos.useQuery();
  return (
    <div>
      {data?.map((repo) => (
        <div key={repo.id}>{repo.name}</div>
      ))}
    </div>
  );
}
