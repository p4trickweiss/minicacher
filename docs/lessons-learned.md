# Lessons Learned

## Technical Decisions & Trade-offs

**Choosing HashiCorp Raft over Custom Implementation**  
Using the HashiCorp Raft library proved to be the right decision for this project. While it abstracted away some of the consensus algorithm's internals, it allowed me to focus on understanding Raft's behavior in practice rather than debugging low-level implementation details. The library's maturity and production-tested nature provided a solid foundation for exploring edge cases and failure scenarios.

**HTTP over gRPC for API Endpoints**  
Initially, I implemented gRPC endpoints, assuming it would be the modern choice for inter-service communication. However, I discovered that the HashiCorp Raft library uses HTTP for its internal transport layer. Maintaining both gRPC and HTTP introduced unnecessary complexity and potential confusion. Switching entirely to HTTP simplified the architecture and reduced the mental overhead of managing two different communication protocols in the same system.

**JavaScript/Bun for Integration Testing**  
Using Bun and JavaScript for integration tests rather than Go's native testing framework was initially driven by familiarity, but it revealed an important advantage: external testing forces you to think from a client's perspective. This approach helped catch issues with the HTTP API design that might have been overlooked if testing was too tightly coupled with the implementation code.

## Challenges Encountered

**Observability Gaps**  
Early in the project, debugging issues was difficult due to insufficient logging and metrics. Understanding what each node was doing during consensus required adding comprehensive structured logging and being able to correlate events across nodes using timestamps and request IDs.

## Key Learnings

**Consensus is About Trade-offs, Not Perfection**  
The most valuable insight was understanding that Raft prioritizes consistency and partition tolerance over availability (CP in CAP theorem). In practice, this meant accepting temporary unavailability during network partitions or leader elections.

**Testing Strategy Matters More Than Test Coverage**  
Rather than achieving high code coverage, focusing on integration tests that simulate realistic failure scenarios (node crashes, network partitions, leadership changes) provided more value. Using Docker-compose to orchestrate multi-node tests was crucial for discovering edge cases that unit tests would never catch.

**The Importance of the State Machine Abstraction**  
Understanding that Raft only handles consensus while the application must implement a deterministic state machine was a key conceptual hurdle. The separation of concerns between "agreeing on the log order" (Raft) and "applying log entries" (cache logic) became clearer through implementation and testing.

## Recommendations for Next Semester

**Expand Chaos Engineering Practices**  
Introduce more sophisticated failure injection: random node crashes, network partitions between specific nodes, slow networks, and disk failures. 

**Focus on Edge Cases Documentation**  
Create a comprehensive test suite that documents discovered edge cases with clear explanations. This would serve both as regression tests and as learning material for understanding Raft's behavior.

**Consider Performance Benchmarking**  
Add benchmarking to understand throughput and latency characteristics under different cluster sizes and workload patterns. This would provide insights into the practical performance implications of strong consistency.

## Conclusion

This project reinforced that distributed systems require a fundamentally different mindset than single-node applications. The decision to prioritize testing and exploration over feature development was valuable—understanding *why* things fail in distributed systems is more important than building feature-rich but unreliable systems. The hands-on experience with Raft, combined with systematic testing, provided insights that theoretical study alone could not deliver.
