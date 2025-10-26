import type { State } from "./state"
import type { Values, ResourceChange } from "./values"
import type { Type } from "./types"
import type { Check } from "./checks"
import type { Configuration } from "./configuration"

/**
 * Plan Representation — The complete top-level object returned by `terraform show -json <STATE FILE>`.
 *
 * A plan consists of a prior state, the configuration that is being applied to that state, and the set of changes Terraform plans to make to achieve that.
 *
 * For ease of consumption by callers, the plan representation includes a partial representation of the values in the final state (using a Value Representation), allowing callers to easily analyze the planned outcome using similar code as for analyzing the prior state.
 */
export interface Plan {
  /**
   * JSON output format version (as of Terraform 1.1.0 this is "1.0").
   */
  format_version: string

  /**
   * A representation of the state that the configuration is being applied to, using the State Representation.
   */
  prior_state?: State

  /**
   * Indicates whether the plan is applyable (wrapping automation should primarily use this flag to decide whether to attempt an apply).
   *
   * Indicates whether it would make sense for a wrapping automation to try to apply this plan, possibly after asking a human operator for approval.
   *
   * Other attributes may give additional context about why the plan is not applyable, but wrapping automations should use this flag as their primary condition to accommodate potential changes to the exact definition of "applyable" in future Terraform versions.
   */
  applyable: boolean

  /**
   * Indicates whether Terraform expects that after applying this plan the actual state will match the desired state.
   *
   * An incomplete plan is expected to require at least one additional plan/apply round to achieve convergence, and so wrapping automations should ideally either automatically start a new plan/apply round after this plan is applied, or prompt the operator that they should do so.
   *
   * Other attributes may give additional context about why the plan is not complete, but wrapping automations should use this flag as their primary condition to accommodate potential changes to the exact definition of "complete" in future Terraform versions.
   */
  complete: boolean

  /**
   * Indicates whether planning failed. An errored plan cannot be applied, but the actions planned before failure may help to understand the error.
   */
  errored: boolean

  /**
   * A representation of the configuration being applied to the prior state, using the Configuration Representation.
   */
  configuration?: Configuration

  /**
   * A description of what is known so far of the outcome in the standard Value Representation, with any as-yet-unknown values omitted.
   */
  planned_values?: Values

  /**
   * A representation of the attributes, including any potentially-unknown attributes. Each value is replaced with "true" or "false" depending on whether it is known in the proposed plan.
   */
  proposed_unknown?: Values

  /**
   * A representation of all the variables provided for the given plan. This is structured as a map similar to the output map so we can add additional fields in later.
   */
  variables?: Record<string, { value: Type }>

  /**
   * Each element of this array describes the action to take for one instance object. All resources in the configuration are included in this list.
   */
  resource_changes: ResourceChange[]

  /**
   * A description of the changes Terraform detected when it compared the most recent state to the prior saved state.
   */
  resource_drift: ResourceChange[]

  /**
   * Lists the sources of all values contributing to changes in the plan. You can use "relevant_attributes" to filter "resource_drift" and determine which external changes may have affected the plan result.
   */
  relevant_attributes: RelevantAttribute[]

  /** Describes the planned changes to the output values of the root module. */
  output_changes: Record<
    string,
    {
      /**
       * Describes the change that will be made to the indicated output value, using the same representation as for resource changes except that the only valid actions values are:
       *
       *   - ["create"]
       *   - ["update"]
       *   - ["delete"]
       *
       * In the Terraform CLI 0.12.0 release, Terraform is not yet fully able to track changes to output values, so the actions indicated may not be fully accurate, but the "after" value will always be correct.
       */
      change: "create" | "update" | "delete"
    }
  >

  /**
   * Describes the partial results for any checkable objects, such as resources with postconditions, with as much information as Terraform can recognize at plan time. Some objects will have status "unknown" to indicate that their status will only be determined after applying the plan.
   */
  checks: Check[]
}

/**
 * An entry in the `relevant_attributes` array that lists which external values contributed to changes in the plan.
 */
export interface RelevantAttribute {
  resource: string
  attribute: string
}
