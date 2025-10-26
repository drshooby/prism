import type { Change } from "./change";
import type { Type, TypeString } from "./types";

/**
 * Values Representation — A sub-object of both plan and state output that describes current state or planned state.
 *
 * A values representation is used in both state and plan output to describe current state (which is always complete) and planned state (which omits values not known until apply).
 */
export interface Values {
  /**
   * Describes the outputs from the root module. Outputs from descendant modules are not available because they are not retained in all of the underlying structures we will build this values representation from.
   */
  outputs?: Record<string, Output>;

  /** Describes the resources and child modules in the root module. */
  root_module?: Module;
}

/** An output value in the Values Representation. */
export interface Output {
  /** The computed or configured value. */
  value?: Type;

  /** Type serialization per Terraform types. */
  type?: TypeString;

  /** Whether this output is marked sensitive. */
  sensitive?: boolean;
}

/** A module in the Values Representation. */
export interface Module {
  /** Resources within this module. */
  resources: Resource[];

  /** Recursively nested child modules. */
  child_modules: ChildModule[];
}

export type ChildModule = Module & {
  /**
   * The absolute module address, which callers must treat as opaque but may do full string comparisons with other module address strings and may pass verbatim to other Terraform commands that are documented as accepting absolute module addresses.
   */
  address: string;
};

/** A resource object inside a Values Representation. */
export interface Resource {
  /**
   * The absolute resource address, which callers must consider opaque but may do full string comparisons with other address strings or pass this verbatim to other Terraform commands that are documented to accept absolute resource addresses. The module-local portions of this address are extracted in other properties.
   */
  address: string;

  /** Resource mode: "managed" for resources, "data" for data sources. */
  mode: "managed" | "data";

  /** Resource type (e.g. "aws_instance"). */
  type: string;

  /** Local resource name. */
  name: string;

  /**
   * If the count or for_each meta-arguments are set for this resource, the additional key "index" is present to give the instance index key. This is omitted for the single instance of a resource that isn't using count or for_each.
   */
  index?: number;

  /**
   * The name of the provider that is responsible for this resource. This is only the provider name, not a provider configuration address, and so no module path nor alias will be indicated here. This is included to allow the property "type" to be interpreted unambiguously in the unusual situation where a provider offers a resource type whose name does not start with its own name, such as the "googlebeta" provider offering "google_compute_instance".
   */
  provider_name: string;

  /** Indicates which version of the resource type schema the "values" property conforms to. */
  schema_version: number;

  /**
   * The JSON representation of the attribute values of the resource, whose structure depends on the resource type schema. Any unknown values are omitted or set to null, making them indistinguishable from absent values; callers which need to distinguish unknown from unset must use the plan-specific or configuration-specific structures.
   */
  values: Record<string, Type>;

  /**
   * The JSON representation of the sensitivity of the resource's attribute values. Only attributes which are sensitive are included in this structure.
   */
  sensitive_values: Record<string, boolean>;
}

/**
 * A description of the individual change actions that Terraform plans to use to move from the prior state to a new state matching the configuration.
 */
export interface ResourceChange {
  /**
   * The full absolute address of the resource instance this change applies to, in the same format as addresses in a Value Representation.
   */
  address: string;

  /**
   * The full absolute address of this resource instance as it was known after the previous Terraform run. Included only if the address has changed, e.g. by handling a "moved" block in the configuration.
   */
  previous_address?: string;

  /**
   * If set, the module portion of the `address` field. Omitted if the instance is in the root module.
   */
  module_address?: string;

  /** Resource mode: "managed" for resources, "data" for data sources. */
  mode: string;

  /** Resource type (e.g. "aws_instance"). */
  type: string;

  /** Local resource name. */
  name: string;

  /**
   * If the count or for_each meta-arguments are set for this resource, the additional key "index" is present to give the instance index key. This is omitted for the single instance of a resource that isn't using count or for_each.
   */
  index?: number;

  /**
   * If set, indicates that this action applies to a "deposed" object of the given instance rather than to its "current" object. Omitted for changes to the current object. "address" and "deposed" together form a unique key across all change objects in a particular plan. The value is an opaque key representing the specific deposed object.
   */
  deposed?: string;

  /** Describes the change that will be made to the indicated object. See the Change Representation. */
  change: Change;

  /**
   * "action_reason" is some optional extra context about why the actions given inside "change" were selected. This is the JSON equivalent of annotations shown in the normal plan output like "is tainted, so must be replaced" as opposed to just "must be replaced".
   *
   * These reason codes are display hints only and the set of possible hints may change over time. Users of this must be prepared to encounter unrecognized reasons and treat them as unspecified reasons.
   *
   * The current set of possible values is:
   * - "replace_because_tainted": the object in question is marked as "tainted" in the prior state, so Terraform planned to replace it.
   * - "replace_because_cannot_update": the provider indicated that one of the requested changes isn't possible without replacing the existing object with a new object.
   * - "replace_by_request": the user explicitly called for this object to be replaced as an option when creating the plan, which therefore overrode what would have been a "no-op" or "update" action otherwise.
   * - "delete_because_no_resource_config": Terraform found no resource configuration corresponding to this instance.
   * - "delete_because_no_module": The resource instance belongs to a module instance that's no longer declared, perhaps due to changing the "count" or "for_each" argument on one of the containing modules.
   * - "delete_because_wrong_repetition": The instance key portion of the resource address isn't of a suitable type for the corresponding resource's configured repetition mode (count, for_each, or neither).
   * - "delete_because_count_index": The corresponding resource uses count, but the instance key is out of range for the currently-configured count value.
   * - "delete_because_each_key": The corresponding resource uses for_each, but the instance key doesn't match any of the keys in the currently-configured for_each value.
   * - "read_because_config_unknown": For a data resource, Terraform cannot read the data during the plan phase because of values in the configuration that won't be known until the apply phase.
   * - "read_because_dependency_pending": For a data resource, Terraform cannot read the data during the plan phase because the data resource depends on at least one managed resource that also has a pending change in the same plan.
   *
   * If there is no special reason to note, Terraform will omit this property altogether.
   */
  action_reason?: string;
}
