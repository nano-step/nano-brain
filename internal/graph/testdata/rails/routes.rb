Rails.application.routes.draw do
  resources :story_statuses

  namespace :api do
    namespace :v1 do
      post "signup" => "tokens#signup"
      get "payments/upcoming-month" => "payments#upcoming_month"
      resources :moments
      resources :payments do
        collection do
          get 'billing'
        end
      end
    end
  end

  resources :users do
    collection do
      get 'token_check'
    end
  end
end
