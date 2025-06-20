setup() {
    : # nothing to set up
}

teardown() {
    : # nothing to tear down
}

@test "validate_api: negative idle cost values" {
    go test ./test/integration/validate_api/idle_cost_negative_test.go
}

@test "validate_api: validate total efficiency" {
   go test ./test/integration/validate_api/valid_total_efficiency_test.go
}

@test "validate_api: validate Total cost vs individual cost" {
   go test ./test/integration/validate_api/individual_costs_sum_vs_totalcost_test.go
}

@test "validate_api: validate Window Start and End" {
    go test ./test/integration/validate_api/correct_window_values_test.go
}

@test "validate_api: validate if all of idle costs are spread" {
    go test ./test/integration/validate_api/share_idle_shares_test.go
}
# Test to validate if all the keys are returned in the API for /allocation

# Test to validate if all the keys are returned in the API for /assets