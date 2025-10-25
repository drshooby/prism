"use client";

import { api } from "@/trpc/react";

export default function TRPCPage() {
  const { data } = api.github.getUserRepos.useQuery();

  const { mutate: createProject } = api.projects.createProject.useMutation({
    onSuccess: (project) => {
      alert(`Project created: ${project?.id}`);
    },
    onError: (error) => {
      alert(`Error creating project: ${error.message}`);
    },
  });

  return (
    <div>
      {data?.map((repo) => (
        <div
          key={repo.id}
          onClick={() =>
            createProject({ repoName: repo.name, owner: repo.owner.login })
          }
        >
          {repo.name}
        </div>
      ))}
    </div>
  );
}
