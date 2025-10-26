// Components
import { SignInOutWrapper } from "@/components/SignInOutWrapper"
import { Dithering, DotGrid, GodRays } from "@paper-design/shaders-react"

// Functions
import { auth } from "@/server/auth"
import { redirect } from "next/navigation"
import { isAuthorized } from "@/server/auth/config"

// Styles
import styles from "./Home.module.css"

// Constants
import { APP_DESCRIPTION, APP_NAME } from "@/lib/constants"

export default async function Home() {
  const session = await auth()
  const authorized = isAuthorized(session)
  if (authorized) redirect("/dashboard")

  return (
    <>
      <main className={styles.page}>
        <GodRays
          style={{
            position: "fixed",
            zIndex: -2,
            opacity: 0.5
          }}
          width={"100vw"}
          height={"100vh"}
          colors={["#005eff"]}
          colorBack="#030408"
          bloom={0}
          intensity={0.88}
          density={0.1}
          spotty={0.3}
          midSize={0.2}
          midIntensity={0.4}
          speed={0.75}
          offsetX={1}
          offsetY={-0.55}
        />
        <DotGrid
          style={{
            position: "fixed",
            zIndex: -1,
            opacity: 0.5
          }}
          width={"100vw"}
          height={"100vh"}
          colorBack="#03040700"
          colorFill="#f8f9fc"
          colorStroke="#ffaa00"
          size={0.5}
          gapX={32}
          gapY={32}
          strokeWidth={0}
          sizeRange={0}
          opacityRange={0}
          shape="circle"
        />
        <div className={styles.content}>
          {/*<Dithering
            width={200}
            height={200}
            colorBack="#03040700"
            colorFront="#F7F9FC"
            shape="sphere"
            type="4x4"
            size={8}
            speed={1}
          />*/}
          <h1 className={styles.title}>{APP_NAME}</h1>
          <p className={styles.description}>{APP_DESCRIPTION}</p>
          <SignInOutWrapper />
        </div>
      </main>
    </>
  )
}
