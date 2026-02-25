Feature: Default branch fallback for niuma worktree creation
  As an automation agent
  I want niuma to resolve remote default branch with multi-level fallbacks
  So implement stage can work on main-only, master-only, and custom-default repositories

  Scenario: main-only repo with missing origin/HEAD
    Given remote repository only has branch "main"
    And local refs/remotes/origin/HEAD is missing
    When niuma creates worktree with empty base
    Then default branch should resolve to "main"
    And worktree creation should succeed
    And niuma should not require origin/master

  Scenario: master-only repo remains backward compatible
    Given remote repository only has branch "master"
    And refs/remotes/origin/HEAD may or may not exist
    When niuma auto creates integration baseline branch
    Then baseline should still be created successfully from master lineage

  Scenario: custom default branch develop
    Given origin HEAD points to "develop"
    When niuma resolves default branch
    Then resolved default branch should be "develop"
    And fetch or checkout should use develop as source

  Scenario: origin/HEAD missing and ls-remote unavailable but origin/main exists locally
    Given symbolic-ref and ls-remote probes are unavailable
    And local refs/remotes/origin/main exists
    When niuma resolves default branch
    Then fallback should choose "main"
    And it should not downgrade to "master"

  Scenario: all probes fail and downstream git operation also fails
    Given origin is unreachable
    And local refs/remotes/origin/main does not exist
    When niuma resolves default branch and tries fetch or checkout
    Then niuma should fallback to "master"
    And the final error should include clear diagnostics
