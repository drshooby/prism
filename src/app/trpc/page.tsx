"use client"

import { api } from "@/trpc/react"

const Page = () => {
  const { data } = api.github.getUserRepos.useQuery()

  const { mutate } = api.projects.createProject.useMutation()

  return (
    <div>
      <div>
        {data?.map((repo) => (
          <div
            key={repo.id}
            onClick={() =>
              mutate({ repoName: repo.name, owner: repo.owner.login })
            }
          >
            {repo.name}
          </div>
        ))}
      </div>
    </div>
  )
}

export default Page
