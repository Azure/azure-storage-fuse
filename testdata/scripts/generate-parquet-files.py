import pandas as pd
import numpy as np
import random

# Function to generate a random number of rows for the DataFrame
def random_row_count(min_rows=10, max_rows=1000):
    return random.randint(min_rows, max_rows)

# Generate 10 Parquet files with varying sizes
for i in range(10):
    row_count = random_row_count()
    df = pd.DataFrame(np.random.randn(row_count, 4), columns=list('ABCD'))
    file_name = f'mixedfiles_{i}.parquet'
    df.to_parquet(file_name, index=False)
    print(f'Created {file_name} with {row_count} rows')
